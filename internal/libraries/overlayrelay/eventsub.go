package overlayrelay

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/twitchsig"
)

// Twitch EventSub webhook headers specific to this relay's handler. The
// signature-related headers (message id, timestamp, signature) live in
// twitchsig, which owns verification.
const (
	HeaderMessageType      = "Twitch-Eventsub-Message-Type"
	HeaderSubscriptionType = "Twitch-Eventsub-Subscription-Type"
)

// Message types Twitch sends to a webhook callback. A notification is handled by
// falling through to ParseNotification, so it needs no constant of its own.
const (
	MessageTypeVerification = "webhook_callback_verification"
	MessageTypeRevocation   = "revocation"
)

// EventSub subscription types this relay forwards.
const (
	SubTypeCheer      = "channel.cheer"
	SubTypeRedemption = "channel.channel_points_custom_reward_redemption.add"
	SubTypePowerUp    = "channel.custom_power_up_redemption.add"
)

// MaxMessageAge bounds replay. A valid signature proves authenticity but says
// nothing about freshness, so a captured notification stays replayable forever
// without this check. Twitch's own guidance is 10 minutes.
const MaxMessageAge = 10 * time.Minute

// IsStale reports whether the message timestamp is outside the replay window in
// either direction. An unparseable timestamp counts as stale: it is covered by
// the signature, so a malformed one means something is wrong regardless.
func IsStale(header http.Header, now time.Time, window time.Duration) bool {
	ts, err := time.Parse(time.RFC3339Nano, header.Get(twitchsig.HeaderMessageTimestamp))
	if err != nil {
		return true
	}
	drift := now.Sub(ts)
	if drift < 0 {
		drift = -drift
	}
	return drift > window
}

// notification is the envelope Twitch wraps every event in.
type notification struct {
	Subscription struct {
		Type      string `json:"type"`
		Condition struct {
			BroadcasterUserID string `json:"broadcaster_user_id"`
		} `json:"condition"`
	} `json:"subscription"`
	Event     json.RawMessage `json:"event"`
	Challenge string          `json:"challenge"`
}

// Challenge extracts the value a verification request wants echoed back.
func Challenge(rawBody []byte) (string, error) {
	var n notification
	if err := json.Unmarshal(rawBody, &n); err != nil {
		return "", fmt.Errorf("decode challenge: %w", err)
	}
	if n.Challenge == "" {
		return "", fmt.Errorf("verification request carried no challenge")
	}
	return n.Challenge, nil
}

// ErrUnsupportedType means the notification verified but carries a subscription
// type no overlay acts on. Callers should acknowledge it, not retry it.
var ErrUnsupportedType = fmt.Errorf("unsupported subscription type")

// ParseNotification converts a verified notification into the shape the log
// stores. The payload is narrowed to the fields an overlay uses rather than
// stored whole, so the wire contract with the overlay does not silently change
// when Twitch adds fields.
func ParseNotification(header http.Header, rawBody []byte) (Incoming, error) {
	var n notification
	if err := json.Unmarshal(rawBody, &n); err != nil {
		return Incoming{}, fmt.Errorf("decode notification: %w", err)
	}

	messageID := header.Get(twitchsig.HeaderMessageID)
	if messageID == "" {
		return Incoming{}, fmt.Errorf("notification carried no message id")
	}
	broadcaster := n.Subscription.Condition.BroadcasterUserID
	if broadcaster == "" {
		return Incoming{}, fmt.Errorf("notification carried no broadcaster_user_id")
	}

	in := Incoming{
		TwitchMessageID:   messageID,
		BroadcasterUserID: broadcaster,
	}

	switch n.Subscription.Type {
	case SubTypeCheer:
		var raw struct {
			UserID      string `json:"user_id"`
			UserLogin   string `json:"user_login"`
			UserName    string `json:"user_name"`
			IsAnonymous bool   `json:"is_anonymous"`
			Bits        int    `json:"bits"`
			Message     string `json:"message"`
		}
		if err := json.Unmarshal(n.Event, &raw); err != nil {
			return Incoming{}, fmt.Errorf("decode cheer event: %w", err)
		}
		payload, err := json.Marshal(CheerPayload(raw))
		if err != nil {
			return Incoming{}, fmt.Errorf("encode cheer payload: %w", err)
		}
		in.Kind = KindCheer
		in.Payload = payload

	case SubTypeRedemption:
		var raw struct {
			UserID    string `json:"user_id"`
			UserLogin string `json:"user_login"`
			UserName  string `json:"user_name"`
			UserInput string `json:"user_input"`
			Status    string `json:"status"`
			Reward    struct {
				ID    string `json:"id"`
				Title string `json:"title"`
				Cost  int    `json:"cost"`
			} `json:"reward"`
		}
		if err := json.Unmarshal(n.Event, &raw); err != nil {
			return Incoming{}, fmt.Errorf("decode redemption event: %w", err)
		}
		payload, err := json.Marshal(RedemptionPayload{
			UserID:      raw.UserID,
			UserLogin:   raw.UserLogin,
			UserName:    raw.UserName,
			RewardID:    raw.Reward.ID,
			RewardTitle: raw.Reward.Title,
			RewardCost:  raw.Reward.Cost,
			UserInput:   raw.UserInput,
			Status:      raw.Status,
		})
		if err != nil {
			return Incoming{}, fmt.Errorf("encode redemption payload: %w", err)
		}
		in.Kind = KindRedemption
		in.Payload = payload

	case SubTypePowerUp:
		// Custom Power-ups (paid with Bits). Confirmed against a live redemption
		// 2026-08-30: user fields are top-level, and the power-up identity + bits
		// are nested under "custom_power_up" (id, title, bits, prompt).
		var raw struct {
			UserID        string `json:"user_id"`
			UserLogin     string `json:"user_login"`
			UserName      string `json:"user_name"`
			CustomPowerUp struct {
				ID    string `json:"id"`
				Title string `json:"title"`
				Bits  int    `json:"bits"`
			} `json:"custom_power_up"`
		}
		if err := json.Unmarshal(n.Event, &raw); err != nil {
			return Incoming{}, fmt.Errorf("decode power-up event: %w", err)
		}
		payload, err := json.Marshal(PowerUpPayload{
			UserID:       raw.UserID,
			UserLogin:    raw.UserLogin,
			UserName:     raw.UserName,
			Bits:         raw.CustomPowerUp.Bits,
			PowerUpID:    raw.CustomPowerUp.ID,
			PowerUpTitle: raw.CustomPowerUp.Title,
		})
		if err != nil {
			return Incoming{}, fmt.Errorf("encode power-up payload: %w", err)
		}
		in.Kind = KindPowerUp
		in.Payload = payload

	default:
		return Incoming{}, fmt.Errorf("%w: %s", ErrUnsupportedType, n.Subscription.Type)
	}

	return in, nil
}
