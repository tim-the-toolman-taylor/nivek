package overlayrelay

import "sync"

// outboxDepth is how far a single overlay may fall behind before we give up on
// it. Cheers arrive a handful per minute at worst, so a client that cannot
// drain 64 frames is wedged, not busy.
const outboxDepth = 64

// Conn is one live overlay connection.
type Conn struct {
	UserID   int
	DeviceID int

	out    chan ServerFrame
	cancel func()

	closeOnce sync.Once
}

// Out is the frame queue the websocket writer drains.
func (c *Conn) Out() <-chan ServerFrame { return c.out }

// Close detaches the connection. Safe to call from either pump and more than
// once.
func (c *Conn) Close() {
	c.closeOnce.Do(func() {
		if c.cancel != nil {
			c.cancel()
		}
	})
}

// Registry tracks which broadcasters currently have an overlay attached.
//
// It is intentionally in-process: with one core-api instance the map IS the
// truth, and reading it is how the dashboard answers "is your overlay running?"
// without a heartbeat table and a TTL to guess at. Running more than one
// instance would need this moved behind Postgres LISTEN/NOTIFY or Redis.
type Registry struct {
	mu sync.RWMutex
	m  map[int]*Conn
}

func NewRegistry() *Registry {
	return &Registry{m: make(map[int]*Conn)}
}

// Add registers a connection for userID, displacing any existing one. One live
// overlay per streamer is the model: a second sign-in -- whether a reconnect
// after an unclean disconnect or a different device -- displaces the previous
// connection, which would otherwise sit there absorbing pushes nothing reads.
// deviceID records which device is attached so the dashboard can report it and
// a revocation can target the right socket.
func (r *Registry) Add(userID, deviceID int, cancel func()) *Conn {
	conn := &Conn{
		UserID:   userID,
		DeviceID: deviceID,
		out:      make(chan ServerFrame, outboxDepth),
		cancel:   cancel,
	}

	r.mu.Lock()
	previous := r.m[userID]
	r.m[userID] = conn
	r.mu.Unlock()

	if previous != nil {
		previous.Close()
	}
	return conn
}

// Remove detaches conn if it is still the registered one. The identity check
// matters: a displaced connection tears down after its replacement has already
// registered, and must not evict it.
func (r *Registry) Remove(conn *Conn) {
	if conn == nil {
		return
	}
	r.mu.Lock()
	if r.m[conn.UserID] == conn {
		delete(r.m, conn.UserID)
	}
	r.mu.Unlock()
	conn.Close()
}

// Push delivers a frame to userID's overlay if one is attached.
//
// A missing or wedged client is not an error: the event is already committed to
// the log, so the overlay picks it up from its cursor on the next connect. What
// must not happen is this blocking -- it runs on the EventSub ingest path, and
// Twitch expects an acknowledgement within seconds.
func (r *Registry) Push(userID int, frame ServerFrame) {
	r.mu.RLock()
	conn := r.m[userID]
	r.mu.RUnlock()
	if conn == nil {
		return
	}

	select {
	case conn.out <- frame:
	default:
		// Backed up past outboxDepth. Drop the connection rather than the
		// event; the client reconnects and replays from its cursor, which is
		// lossless, whereas skipping a frame here would not be.
		conn.Close()
	}
}

// IsConnected reports whether userID has an overlay attached.
func (r *Registry) IsConnected(userID int) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.m[userID]
	return ok
}

// ConnectedDeviceID returns the device id of userID's attached overlay, if any.
// The dashboard uses it to mark which of a streamer's registered devices is
// currently live.
func (r *Registry) ConnectedDeviceID(userID int) (int, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	conn, ok := r.m[userID]
	if !ok {
		return 0, false
	}
	return conn.DeviceID, true
}

// DisconnectDevice tears down userID's live connection only if it is the given
// device. Revocation calls this so a revoked token's open socket stops
// immediately, rather than lingering until it next reconnects -- but a
// different, still-valid device of the same user keeps its connection.
func (r *Registry) DisconnectDevice(userID, deviceID int) {
	r.mu.RLock()
	conn, ok := r.m[userID]
	r.mu.RUnlock()
	if ok && conn.DeviceID == deviceID {
		conn.Close()
	}
}

// Count is the number of attached overlays, for logging and health output.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.m)
}
