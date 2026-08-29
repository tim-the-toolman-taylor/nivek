package overlay

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/labstack/echo/v4"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/overlayrelay"
)

const (
	// helloTimeout is how long an accepted socket may stay anonymous. Without
	// it an unauthenticated peer could hold a connection open indefinitely.
	helloTimeout = 10 * time.Second

	// pingInterval keeps NAT and proxy state alive on a link that is idle
	// between cheers, and surfaces a half-open connection instead of leaving
	// the overlay believing it is still attached.
	pingInterval = 30 * time.Second

	writeTimeout = 10 * time.Second

	// maxReplayPerConn bounds how many events a single connection will replay
	// before handing back to a reconnect. overlayrelay.MaxReplay is the page
	// size; this is the total, so an overlay that has been offline a very long
	// time (or always sends since=0) cannot stream the whole log down one socket.
	// The overlay advances its cursor as it processes, so continuing on reconnect
	// is lossless. Cheers arrive a handful per minute, so this is days of backlog.
	maxReplayPerConn = 5000
)

// NewConnectEndpoint upgrades to a websocket and streams a broadcaster's events
// to their overlay.
//
// Authentication is a device token in the first frame rather than a header or
// cookie: the client is a desktop application, not a browser, and this keeps
// the credential out of request logs and proxy access logs.
func NewConnectEndpoint(svc nivek.NivekService, relay overlayrelay.Service, registry *overlayrelay.Registry) echo.HandlerFunc {
	return func(c echo.Context) error {
		// core-api sets ReadTimeout/WriteTimeout on its http.Server for the
		// REST surface. Those deadlines are already on the underlying
		// connection when the handler runs, and websocket.Accept hijacks
		// without clearing them -- so a long-lived socket would die roughly 30
		// seconds in, for no visible reason. Clear them here rather than
		// weakening the timeouts for every other route.
		rc := http.NewResponseController(c.Response())
		if err := rc.SetReadDeadline(time.Time{}); err != nil {
			svc.Logger().Errorf("overlay connect: clear read deadline: %s", err.Error())
			return c.NoContent(http.StatusInternalServerError)
		}
		if err := rc.SetWriteDeadline(time.Time{}); err != nil {
			svc.Logger().Errorf("overlay connect: clear write deadline: %s", err.Error())
			return c.NoContent(http.StatusInternalServerError)
		}

		conn, err := websocket.Accept(c.Response(), c.Request(), &websocket.AcceptOptions{
			// The overlay is a native client and sends no Origin header, so
			// there is nothing to check. Any browser-based client added later
			// WILL send one, and will need a real OriginPatterns list here.
			InsecureSkipVerify: true,
		})
		if err != nil {
			svc.Logger().Debugf("overlay connect: accept failed: %s", err.Error())
			return nil
		}
		defer conn.CloseNow() //nolint:errcheck // best effort teardown

		ctx, cancel := context.WithCancel(c.Request().Context())
		defer cancel()

		device, hello, err := readHello(ctx, conn, relay)
		if err != nil {
			svc.Logger().Debugf("overlay connect: handshake failed: %s", err.Error())
			_ = conn.Close(websocket.StatusPolicyViolation, "handshake failed")
			return nil
		}

		registered := registry.Add(device.UserId, device.Id, cancel)
		defer registry.Remove(registered)

		svc.Logger().Infof("overlay connected: user=%d device=%d since=%d (%d attached)",
			device.UserId, device.Id, hello.Since, registry.Count())
		defer func() {
			svc.Logger().Infof("overlay disconnected: user=%d device=%d", device.UserId, device.Id)
		}()

		// Start the reader before replaying. coder/websocket needs a goroutine
		// draining the connection to process control frames, so during a long
		// catch-up the overlay's pings and any close must not sit unhandled; it
		// also lets a peer that goes away mid-replay cancel the context and end
		// the writes below.
		go readPump(ctx, cancel, conn)

		// Replay before announcing ready, so the overlay never interleaves
		// backlog with live events and can treat the stream as ordered. Page
		// through the backlog in MaxReplay-sized batches until caught up (a single
		// capped read would silently truncate an overlay that missed more than one
		// page), but bound the total per connection: on hitting the budget we stop
		// short of "ready" and close, and the overlay -- whose cursor has advanced
		// over what we sent -- reconnects and continues from there, losslessly.
		//
		// cursor drives paging and respects the client's reported position.
		// lastReplayedSeq is the dedup threshold below; it starts at 0 (not
		// hello.Since) and advances only over events we actually replay, so an
		// over-reported since -- a corrupted or ahead-of-truth cursor -- cannot
		// suppress genuine live events sitting below it.
		cursor := hello.Since
		var lastReplayedSeq int64
		replayed := 0

		// The live outbox was attached before replay, so live events accumulate
		// there while we page. Drain it as we go -- buffering, not sending, to keep
		// the stream ordered -- so a burst during a long replay can't overflow the
		// 64-slot outbox and force a reconnect. Buffered frames are flushed in
		// order (deduped) right after "ready".
		var pendingLive []overlayrelay.ServerFrame
		drainOutbox := func() {
			for {
				select {
				case frame := <-registered.Out():
					pendingLive = append(pendingLive, frame)
				default:
					return
				}
			}
		}

		for {
			backlog, err := relay.EventsAfter(device.UserId, cursor, overlayrelay.MaxReplay)
			if err != nil {
				svc.Logger().Errorf("overlay connect: replay for user %d: %s", device.UserId, err.Error())
				_ = conn.Close(websocket.StatusInternalError, "replay failed")
				return nil
			}
			for i := range backlog {
				if err := writeFrame(ctx, conn, overlayrelay.ServerFrame{
					Type:  overlayrelay.MsgEvent,
					Event: &backlog[i],
				}); err != nil {
					return nil
				}
				cursor = backlog[i].Seq
				lastReplayedSeq = backlog[i].Seq
				replayed++
				drainOutbox()
			}
			// A short page means the cursor has reached the tail. Anything
			// committed after this point arrives on the live outbox instead.
			if len(backlog) < overlayrelay.MaxReplay {
				break
			}
			if replayed >= maxReplayPerConn {
				svc.Logger().Infof("overlay connect: replay budget (%d) reached for user %d at seq %d; closing to continue on reconnect",
					maxReplayPerConn, device.UserId, lastReplayedSeq)
				_ = conn.Close(websocket.StatusTryAgainLater, "replay budget reached, reconnect to continue")
				return nil
			}
		}
		if err := writeFrame(ctx, conn, overlayrelay.ServerFrame{Type: overlayrelay.MsgReady}); err != nil {
			return nil
		}

		// Flush events that arrived during replay, in order, dropping any that the
		// backlog already covered (same dedup rule as the live loop below). Seq is
		// monotonic per user, so these sit before anything the live loop will read.
		drainOutbox()
		for _, frame := range pendingLive {
			if frame.Type == overlayrelay.MsgEvent && frame.Event != nil && frame.Event.Seq <= lastReplayedSeq {
				continue
			}
			if err := writeFrame(ctx, conn, frame); err != nil {
				return nil
			}
		}

		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return nil

			case frame := <-registered.Out():
				// registry.Add attached the live outbox before the replay above,
				// so an event committed in that window is in both streams. Seq is
				// monotonic per user, so drop any live event at or below the last
				// one we replayed; without this it would be delivered twice and
				// out of order. Non-event frames carry no seq and always pass.
				if frame.Type == overlayrelay.MsgEvent && frame.Event != nil && frame.Event.Seq <= lastReplayedSeq {
					continue
				}
				if err := writeFrame(ctx, conn, frame); err != nil {
					return nil
				}

			case <-ticker.C:
				pingCtx, pingCancel := context.WithTimeout(ctx, writeTimeout)
				err := conn.Ping(pingCtx)
				pingCancel()
				if err != nil {
					return nil
				}
			}
		}
	}
}

// readHello consumes and authenticates the first frame.
func readHello(ctx context.Context, conn *websocket.Conn, relay overlayrelay.Service) (*overlayrelay.Device, overlayrelay.Hello, error) {
	helloCtx, cancel := context.WithTimeout(ctx, helloTimeout)
	defer cancel()

	_, raw, err := conn.Read(helloCtx)
	if err != nil {
		return nil, overlayrelay.Hello{}, err
	}

	var hello overlayrelay.Hello
	if err := json.Unmarshal(raw, &hello); err != nil {
		return nil, overlayrelay.Hello{}, err
	}
	if hello.Type != overlayrelay.MsgHello {
		return nil, overlayrelay.Hello{}, errors.New("first frame was not a hello")
	}
	if hello.Since < 0 {
		hello.Since = 0
	}

	device, err := relay.AuthenticateDevice(hello.Token)
	if err != nil {
		return nil, overlayrelay.Hello{}, err
	}
	return device, hello, nil
}

// readPump drains client frames. Acks are advisory -- the overlay's cursor on
// reconnect is what actually determines delivery -- so nothing here needs to
// reach the database; the value is detecting a dead peer.
func readPump(ctx context.Context, cancel context.CancelFunc, conn *websocket.Conn) {
	defer cancel()
	for {
		_, raw, err := conn.Read(ctx)
		if err != nil {
			return
		}

		var frame overlayrelay.ClientFrame
		if err := json.Unmarshal(raw, &frame); err != nil {
			continue
		}
		if frame.Type == overlayrelay.MsgPing {
			pongCtx, pongCancel := context.WithTimeout(ctx, writeTimeout)
			err := writeFrame(pongCtx, conn, overlayrelay.ServerFrame{Type: overlayrelay.MsgPong})
			pongCancel()
			if err != nil {
				return
			}
		}
	}
}

func writeFrame(ctx context.Context, conn *websocket.Conn, frame overlayrelay.ServerFrame) error {
	payload, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	writeCtx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	return conn.Write(writeCtx, websocket.MessageText, payload)
}
