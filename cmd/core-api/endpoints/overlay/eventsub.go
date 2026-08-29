package overlay

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/coreconfig"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/overlayrelay"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/twitchsig"
)

// maxWebhookBody bounds what we buffer before verifying. EventSub notifications
// are a few kilobytes; anything near this is not one.
const maxWebhookBody = 256 << 10

// NewEventSubEndpoint receives Twitch EventSub notifications for the overlay
// relay.
//
// This is a second, independent callback: the bot already owns /eventsub with
// its own secret and subscription set, and nothing here touches it. Keeping
// them separate means the relay's subscriptions can be created, revoked, and
// have their secret rotated without disturbing chat.
//
// The route is public by necessity -- Twitch authenticates by signing the
// message, not by carrying a session -- so it must stay outside both the JWT
// middleware and the credentialed CORS policy.
func NewEventSubEndpoint(svc nivek.NivekService, relay overlayrelay.Service, registry *overlayrelay.Registry) echo.HandlerFunc {
	// Serialises append+push per broadcaster across concurrent deliveries so live
	// frames reach the outbox in seq order. Shared across all requests to this
	// handler.
	ingestLocks := overlayrelay.NewKeyedMutex()

	return func(c echo.Context) error {
		cfg, ok := svc.CustomConfig().(coreconfig.CoreAPIConfig)
		if !ok {
			return c.NoContent(http.StatusInternalServerError)
		}
		if cfg.OverlayEventSubSecret == "" {
			svc.Logger().Warn("overlay eventsub called but OVERLAY_EVENTSUB_SECRET is unset")
			return c.NoContent(http.StatusServiceUnavailable)
		}

		// The signature covers the exact bytes received, so the body must be
		// read raw and verified before anything parses or re-marshals it.
		raw, err := io.ReadAll(io.LimitReader(c.Request().Body, maxWebhookBody))
		if err != nil {
			return c.NoContent(http.StatusBadRequest)
		}

		header := c.Request().Header
		if !twitchsig.Verify(header, raw, cfg.OverlayEventSubSecret) {
			// Deliberately terse: a caller who cannot sign gets no detail about
			// why, and this is the one route an unauthenticated internet can
			// reach.
			return c.NoContent(http.StatusForbidden)
		}
		if overlayrelay.IsStale(header, time.Now().UTC(), overlayrelay.MaxMessageAge) {
			return c.NoContent(http.StatusForbidden)
		}

		switch header.Get(overlayrelay.HeaderMessageType) {
		case overlayrelay.MessageTypeVerification:
			challenge, err := overlayrelay.Challenge(raw)
			if err != nil {
				svc.Logger().Errorf("overlay eventsub verification: %s", err.Error())
				return c.NoContent(http.StatusBadRequest)
			}
			// Twitch wants the challenge echoed as raw text, byte for byte.
			return c.String(http.StatusOK, challenge)

		case overlayrelay.MessageTypeRevocation:
			svc.Logger().Warnf("overlay eventsub subscription revoked: type=%s", header.Get(overlayrelay.HeaderSubscriptionType))
			return c.NoContent(http.StatusNoContent)
		}

		incoming, err := overlayrelay.ParseNotification(header, raw)
		if err != nil {
			if errors.Is(err, overlayrelay.ErrUnsupportedType) {
				// Acknowledge: retrying will not make us support it, and an
				// error response counts toward Twitch disabling the
				// subscription.
				svc.Logger().Debugf("overlay eventsub: %s", err.Error())
				return c.NoContent(http.StatusNoContent)
			}
			svc.Logger().Errorf("overlay eventsub parse: %s", err.Error())
			return c.NoContent(http.StatusBadRequest)
		}

		// Persist and fan out synchronously. Twitch allows a few seconds and
		// this is one indexed insert; doing it inline means a 2xx genuinely
		// promises the event is durable, so Twitch's retry remains a real
		// safety net rather than something we have already discarded.
		//
		// Hold the broadcaster's ingest lock across append+push so concurrent
		// deliveries for the same channel push in seq order; releasing only after
		// the push (not just the append) is what keeps the outbox ordered.
		unlock := ingestLocks.Lock(incoming.BroadcasterUserID)
		defer unlock()

		event, isNew, err := relay.Ingest(incoming)
		if err != nil {
			if errors.Is(err, overlayrelay.ErrUnknownBroadcaster) {
				svc.Logger().Warnf("overlay eventsub: %s", err.Error())
				return c.NoContent(http.StatusNoContent)
			}
			// 5xx asks Twitch to retry, which is what we want for a transient
			// database failure.
			svc.Logger().Errorf("overlay eventsub ingest: %s", err.Error())
			return c.NoContent(http.StatusInternalServerError)
		}

		if isNew {
			registry.Push(event.UserId, overlayrelay.ServerFrame{
				Type:  overlayrelay.MsgEvent,
				Event: &event,
			})
		}

		return c.NoContent(http.StatusNoContent)
	}
}
