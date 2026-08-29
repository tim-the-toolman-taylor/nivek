package overlayrelay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Twitch EventSub webhook headers.
const (
	HeaderMessageID        = "Twitch-Eventsub-Message-Id"
	HeaderMessageTimestamp = "Twitch-Eventsub-Message-Timestamp"
	HeaderMessageSignature = "Twitch-Eventsub-Message-Signature"
	HeaderMessageType      = "Twitch-Eventsub-Message-Type"
	HeaderSubscriptionType = "Twitch-Eventsub-Subscription-Type"
)

// Message types Twitch sends to a webhook callback.
const (
	MessageTypeVerification = "webhook_callback_verification"
	MessageTypeNotification = "notification"
	MessageTypeRevocation   = "revocation"
)

// EventSub subscription types this relay forwards.
const (
	SubTypeCheer      = "channel.cheer"
	SubTypeRedemption = "channel.channel_points_custom_reward_redemption.add"
)

// MaxMessageAge bounds replay. A valid signature proves authenticity but says
// nothing about freshness, so a captured notification stays replayable forever
// without this check. Twitch's own guidance is 10 minutes.
const MaxMessageAge = 10 * time.Minute

// VerifySignature checks the HMAC Twitch computes over
// messageID + timestamp + rawBody. rawBody must be the exact bytes received:
// re-marshalling parsed JSON will not reproduce the signature.
func VerifySignature(header http.Header, rawBody []byte, secret string) bool {
	if secret == "" {
		return false
	}
	got := header.Get(HeaderMessageSignature)
	if !strings.HasPrefix(got, "sha256=") {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(header.Get(HeaderMessageID)))
	mac.Write([]byte(header.Get(HeaderMessageTimestamp)))
	mac.Write(rawBody)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(got), []byte(want))
}

// IsStale reports whether the message timestamp is outside the replay window in
// either direction. An unparseable timestamp counts as stale: it is covered by
// the signature, so a malformed one means something is wrong regardless.
func IsStale(header http.Header, now time.Time, window time.Duration) bool {
	ts, err := time.Parse(time.RFC3339Nano, header.Get(HeaderMessageTimestamp))
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

	messageID := header.Get(HeaderMessageID)
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

	default:
		return Incoming{}, fmt.Errorf("%w: %s", ErrUnsupportedType, n.Subscription.Type)
	}

	return in, nil
}
