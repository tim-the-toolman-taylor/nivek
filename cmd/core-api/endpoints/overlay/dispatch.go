package overlay

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/overlayrelay"
)

// maxDedupeKey matches nivek.overlay_event.twitch_message_id (VARCHAR(64)).
// The bot builds the key as "<chat message id>:<action>"; a Twitch message id
// is a 36-character UUID, so the longest real key is comfortably inside this.
// Rejecting rather than truncating keeps two different commands from colliding
// into one row and silently losing the second.
const maxDedupeKey = 64

// maxCommandArgs bounds what a chatter can push through in one command. The
// overlay's own commands take at most one argument; this is slack, not a limit
// anyone should reach.
const maxCommandArgs = 8

type dispatchRequest struct {
	BroadcasterUserID string   `json:"broadcaster_user_id"`
	DedupeKey         string   `json:"dedupe_key"`
	Action            string   `json:"action"`
	Args              []string `json:"args,omitempty"`
	UserID            string   `json:"user_id,omitempty"`
	UserLogin         string   `json:"user_login"`
	UserName          string   `json:"user_name"`
}

// NewDispatchEndpoint routes a chat command to a broadcaster's overlay.
//
// The bot owns the decision to send: it has already matched the trigger,
// checked min_role, and confirmed the channel holds the capability the command
// requires. This endpoint's job is to get the command onto the durable log and
// out to the socket, exactly like an EventSub notification -- the difference is
// only in where it came from and that it is never replayed (see EventsAfter).
//
// botAuth-gated. It writes to a user's event log on the bot's say-so, so it
// must never be reachable without the shared HMAC.
func NewDispatchEndpoint(
	svc nivek.NivekService,
	relay overlayrelay.Service,
	registry *overlayrelay.Registry,
) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req dispatchRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "malformed body"})
		}

		req.BroadcasterUserID = strings.TrimSpace(req.BroadcasterUserID)
		req.Action = strings.TrimSpace(req.Action)
		req.DedupeKey = strings.TrimSpace(req.DedupeKey)

		if req.BroadcasterUserID == "" || req.Action == "" || req.DedupeKey == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "broadcaster_user_id, action and dedupe_key are required",
			})
		}
		if len(req.DedupeKey) > maxDedupeKey {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "dedupe_key too long",
			})
		}
		if len(req.Args) > maxCommandArgs {
			req.Args = req.Args[:maxCommandArgs]
		}

		payload, err := json.Marshal(overlayrelay.CommandPayload{
			Action:    req.Action,
			Args:      req.Args,
			UserID:    req.UserID,
			UserLogin: req.UserLogin,
			UserName:  req.UserName,
		})
		if err != nil {
			svc.Logger().Errorf("overlay dispatch: marshal payload: %s", err.Error())
			return c.NoContent(http.StatusInternalServerError)
		}

		event, isNew, err := relay.Ingest(overlayrelay.Incoming{
			TwitchMessageID:   req.DedupeKey,
			BroadcasterUserID: req.BroadcasterUserID,
			Kind:              overlayrelay.KindCommand,
			Payload:           payload,
		})
		if err != nil {
			if errors.Is(err, overlayrelay.ErrUnknownBroadcaster) {
				// The bot gates on a device existing, so this means the account
				// went away between the gate and here. Nothing to deliver to.
				svc.Logger().Warnf("overlay dispatch: %s", err.Error())
				return c.JSON(http.StatusOK, map[string]bool{"delivered": false})
			}
			svc.Logger().Errorf("overlay dispatch ingest: %s", err.Error())
			return c.NoContent(http.StatusInternalServerError)
		}

		if !isNew {
			// Same chat message delivered twice. The command already ran.
			return c.JSON(http.StatusOK, map[string]bool{"delivered": false})
		}

		delivered := registry.IsConnected(event.UserId)
		registry.Push(event.UserId, overlayrelay.ServerFrame{
			Type:  overlayrelay.MsgEvent,
			Event: &event,
		})

		// delivered reports whether an overlay was attached, not whether it
		// acted. A command sent to a closed overlay is dropped on purpose --
		// commands are not replayed.
		return c.JSON(http.StatusOK, map[string]bool{"delivered": delivered})
	}
}
