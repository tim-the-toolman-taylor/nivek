package overlay

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/coreconfig"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/overlayrelay"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/twitchext"
)

// maxExtensionBody bounds what we buffer. Two JWTs plus JSON envelope are a few
// kilobytes; this is generous.
const maxExtensionBody = 64 << 10

// extensionRequest is what the extension panel posts after a Bits purchase. The
// receipt (onTransactionComplete) proves the purchase and names the product;
// the viewer's identity token (onAuthorized), sent as a bearer, names the
// channel -- a Bits receipt does not carry one.
type extensionRequest struct {
	Receipt string `json:"receipt"`
}

// NewExtensionEndpoint receives Bits-in-Extensions interactions from the
// extension panel and feeds them into the same relay the EventSub webhook does.
//
// Trust comes from two JWTs signed with the extension's shared secret: an
// attacker cannot forge either without it. The receipt is the proof of payment
// (SKU + transaction id); the identity token supplies the channel. Because the
// receipt itself names no channel, a viewer could in principle pair a real
// receipt with a different channel's identity token -- but that only spends
// their own Bits to fire an effect on the wrong channel, so it is accepted for
// now rather than guarded.
//
// Public by necessity: the caller is a viewer's browser holding a Twitch-issued
// token, not a dashboard session. It stays outside the JWT middleware; CORS for
// the extension origin is opened in the server setup.
func NewExtensionEndpoint(svc nivek.NivekService, relay overlayrelay.Service, registry *overlayrelay.Registry) echo.HandlerFunc {
	// Serialises append+push per broadcaster so live frames reach the outbox in
	// seq order, exactly as the eventsub handler does. Shared across requests.
	ingestLocks := overlayrelay.NewKeyedMutex()

	return func(c echo.Context) error {
		cfg, ok := svc.CustomConfig().(coreconfig.CoreAPIConfig)
		if !ok {
			return c.NoContent(http.StatusInternalServerError)
		}
		if !cfg.ExtensionEnabled() {
			svc.Logger().Warn("overlay extension called but OVERLAY_EXTENSION_* is unset")
			return c.NoContent(http.StatusServiceUnavailable)
		}

		secret, err := twitchext.DecodeSecret(cfg.OverlayExtensionSecret)
		if err != nil {
			// Validate() already guards this at boot; treat a bad secret here as a
			// server misconfiguration, not a client error.
			svc.Logger().Errorf("overlay extension secret: %s", err.Error())
			return c.NoContent(http.StatusInternalServerError)
		}

		identityToken := bearerToken(c.Request().Header.Get(echo.HeaderAuthorization))
		if identityToken == "" {
			return c.NoContent(http.StatusUnauthorized)
		}

		body, err := io.ReadAll(io.LimitReader(c.Request().Body, maxExtensionBody))
		if err != nil {
			return c.NoContent(http.StatusBadRequest)
		}
		var req extensionRequest
		if err := json.Unmarshal(body, &req); err != nil || strings.TrimSpace(req.Receipt) == "" {
			return c.NoContent(http.StatusBadRequest)
		}

		identity, err := twitchext.VerifyIdentity(identityToken, secret)
		if err != nil {
			// The response stays terse (no body), but log the reason at warning:
			// a 403 here means a token failed to verify, which an operator wants
			// to see -- it is how we distinguish a bad secret from a bad payload.
			svc.Logger().Warnf("overlay extension identity verify failed: %s", err.Error())
			return c.NoContent(http.StatusForbidden)
		}
		receipt, err := twitchext.VerifyReceipt(req.Receipt, secret)
		if err != nil {
			svc.Logger().Warnf("overlay extension receipt verify failed: %s", err.Error())
			return c.NoContent(http.StatusForbidden)
		}

		payload, err := json.Marshal(overlayrelay.ExtensionPayload{
			UserID:      receipt.Data.UserID,
			Bits:        receipt.Data.Product.Cost.Amount,
			ProductSKU:  receipt.Data.Product.SKU,
			ProductName: receipt.Data.Product.DisplayName,
		})
		if err != nil {
			return c.NoContent(http.StatusInternalServerError)
		}

		incoming := overlayrelay.Incoming{
			// The transaction id is Twitch's own idempotency key: the existing
			// (user_id, twitch_message_id) unique constraint dedups a re-posted
			// receipt so an interaction fires exactly once.
			TwitchMessageID:   receipt.Data.TransactionID,
			BroadcasterUserID: identity.ChannelID,
			Kind:              overlayrelay.KindExtension,
			Payload:           payload,
		}

		svc.Logger().Infof(
			"overlay extension: sku=%s bits=%d channel=%s tx=%s",
			receipt.Data.Product.SKU, receipt.Data.Product.Cost.Amount, identity.ChannelID, receipt.Data.TransactionID,
		)

		unlock := ingestLocks.Lock(incoming.BroadcasterUserID)
		defer unlock()

		event, isNew, err := relay.Ingest(incoming)
		if err != nil {
			if errors.Is(err, overlayrelay.ErrUnknownBroadcaster) {
				// The broadcaster has no local account/overlay: nothing to deliver
				// to, but the purchase was valid, so acknowledge it.
				svc.Logger().Warnf("overlay extension: %s", err.Error())
				return c.NoContent(http.StatusNoContent)
			}
			svc.Logger().Errorf("overlay extension ingest: %s", err.Error())
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

// bearerToken pulls the token out of an "Authorization: Bearer <token>" header,
// returning "" if the header is absent or not a bearer.
func bearerToken(header string) string {
	const prefix = "bearer "
	if len(header) < len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}
