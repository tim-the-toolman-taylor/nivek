package twitcheventsub

import (
	"net/http"
	"time"
)

const (
	tokenURL                 = "https://id.twitch.tv/oauth2/token"
	eventSubSubscriptionsURL = "https://api.twitch.tv/helix/eventsub/subscriptions"
	streamsURL               = "https://api.twitch.tv/helix/streams"
	TwitchUsersURL           = "https://api.twitch.tv/helix/users"
	defaultHTTPTimeout       = 10 * time.Second
	// Refresh a minute early so we don't race the exact expiry second.
	appTokenExpirySkew = time.Minute
	HttpTimeout        = 10 * time.Second

	// StatusEnabled is the only healthy EventSub subscription status; any other
	// value (webhook_callback_verification_failed, notification_failures_exceeded,
	// authorization_revoked, …) means Twitch will not deliver notifications.
	StatusEnabled = "enabled"
)

// Config holds Twitch app credentials and EventSub transport settings.
type Config struct {
	ClientID          string
	ClientSecret      string
	EventSubSecret    string
	CallbackURL       string // defaults to DefaultCallbackURL
	HTTPClientTimeout time.Duration
}

// EventSubSubscription is one subscription as returned by Get EventSub
// Subscriptions. Only the fields we audit are decoded.
type EventSubSubscription struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Type      string `json:"type"`
	Version   string `json:"version"`
	Condition struct {
		BroadcasterUserID string `json:"broadcaster_user_id"`
	} `json:"condition"`
	Transport struct {
		Method   string `json:"method"`
		Callback string `json:"callback"`
	} `json:"transport"`
	CreatedAt string `json:"created_at"`
}

type subscriptionPayload struct {
	Type      string `json:"type"`
	Version   string `json:"version"`
	Condition struct {
		BroadcasterUserID string  `json:"broadcaster_user_id"`
		UserId            *string `json:"user_id,omitempty"`
	} `json:"condition"`
	Transport struct {
		Method   string `json:"method"`
		Callback string `json:"callback"`
		Secret   string `json:"secret"`
	} `json:"transport"`
}

// SubscribeResult is the Helix response for one create-subscription call.
type SubscribeResult struct {
	StatusCode int
	Body       []byte
}

// AlreadyExists reports whether Helix indicated the subscription is already present (409).
func (r SubscribeResult) AlreadyExists() bool {
	return r.StatusCode == http.StatusConflict
}

// OK reports 202 Accepted or 200 OK.
func (r SubscribeResult) OK() bool {
	return r.StatusCode == http.StatusAccepted || r.StatusCode == http.StatusOK
}
