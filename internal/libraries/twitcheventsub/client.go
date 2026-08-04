// Package twitcheventsub creates EventSub webhook subscriptions (Helix) and
// mints/caches Twitch app access tokens for that purpose.
//
// Webhook creates require an app access token, not a user token:
// https://dev.twitch.tv/docs/eventsub/manage-subscriptions/#authorization
package twitcheventsub

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/api"
)

const (
	tokenURL                 = "https://id.twitch.tv/oauth2/token"
	eventSubSubscriptionsURL = "https://api.twitch.tv/helix/eventsub/subscriptions"
	streamsURL               = "https://api.twitch.tv/helix/streams"
	defaultHTTPTimeout       = 10 * time.Second
	// Refresh a minute early so we don't race the exact expiry second.
	appTokenExpirySkew = time.Minute
)

// Config holds Twitch app credentials and EventSub transport settings.
type Config struct {
	ClientID          string
	ClientSecret      string
	EventSubSecret    string
	CallbackURL       string // defaults to DefaultCallbackURL
	HTTPClientTimeout time.Duration
}

// Client mints app tokens and creates EventSub webhook subscriptions.
type Client struct {
	cfg        Config
	httpClient *http.Client

	tokenMu     sync.Mutex
	token       string
	tokenExpiry time.Time
}

// NewClient returns a Client. ClientID, ClientSecret, and EventSubSecret are required.
func NewClient(cfg Config) (*Client, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, errors.New("TWITCH_CLIENT_ID and TWITCH_CLIENT_SECRET are required")
	}
	if cfg.EventSubSecret == "" {
		return nil, errors.New("TWITCH_EVENTSUB_SECRET is required")
	}
	if cfg.CallbackURL == "" {
		cfg.CallbackURL = fmt.Sprintf("%s%s", "https://peanutbudderbot.com", api.TwitchWebhookSubscriptionRequest)
	}
	timeout := cfg.HTTPClientTimeout
	if timeout <= 0 {
		timeout = defaultHTTPTimeout
	}
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

// AppAccessToken returns a cached app access token, minting one if needed.
// https://dev.twitch.tv/docs/authentication/getting-tokens-oauth/#client-credentials-grant-flow
func (c *Client) AppAccessToken(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	if c.token != "" && time.Now().Before(c.tokenExpiry.Add(-appTokenExpirySkew)) {
		return c.token, nil
	}

	token, expiresIn, err := c.fetchAppAccessToken(ctx)
	if err != nil {
		return "", err
	}
	c.token = token
	c.tokenExpiry = time.Now().Add(time.Duration(expiresIn) * time.Second)
	return c.token, nil
}

// InvalidateAppAccessToken drops the cache (e.g. after Helix 401).
func (c *Client) InvalidateAppAccessToken() {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	c.token = ""
	c.tokenExpiry = time.Time{}
}

func (c *Client) fetchAppAccessToken(ctx context.Context) (token string, expiresIn int, err error) {
	form := url.Values{}
	form.Set("client_id", c.cfg.ClientID)
	form.Set("client_secret", c.cfg.ClientSecret)
	form.Set("grant_type", "client_credentials")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, fmt.Errorf("building app token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("app token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("reading app token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("app token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var parsed struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", 0, fmt.Errorf("decoding app token response: %w", err)
	}
	if parsed.AccessToken == "" {
		return "", 0, errors.New("app token response missing access_token")
	}
	if parsed.ExpiresIn <= 0 {
		parsed.ExpiresIn = int((24 * time.Hour).Seconds())
	}
	return parsed.AccessToken, parsed.ExpiresIn, nil
}

// IsStreamLive reports whether the broadcaster is currently live, via Helix
// Get Streams. A non-empty data array means an active stream. Used at signup to
// catch users who are already streaming — EventSub only fires on the live
// transition, so it would otherwise miss them.
// Retries once after invalidating the app token cache if Helix returns 401.
// https://dev.twitch.tv/docs/api/reference/#get-streams
func (c *Client) IsStreamLive(ctx context.Context, broadcasterUserID string) (bool, error) {
	if broadcasterUserID == "" {
		return false, errors.New("broadcaster user id is required")
	}

	q := url.Values{}
	q.Set("user_id", broadcasterUserID)
	reqURL := streamsURL + "?" + q.Encode()

	for attempt := 0; attempt < 2; attempt++ {
		appToken, err := c.AppAccessToken(ctx)
		if err != nil {
			return false, fmt.Errorf("app token: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return false, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+appToken)
		req.Header.Set("Client-Id", c.cfg.ClientID)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return false, fmt.Errorf("request: %w", err)
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return false, fmt.Errorf("read response: %w", readErr)
		}

		if resp.StatusCode == http.StatusUnauthorized && attempt == 0 {
			c.InvalidateAppAccessToken()
			continue
		}
		if resp.StatusCode != http.StatusOK {
			return false, fmt.Errorf("get streams returned %d: %s", resp.StatusCode, string(body))
		}

		var parsed struct {
			Data []struct {
				ID   string `json:"id"`
				Type string `json:"type"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &parsed); err != nil {
			return false, fmt.Errorf("decode streams response: %w", err)
		}
		return len(parsed.Data) > 0, nil
	}
	return false, nil
}

type subscriptionPayload struct {
	Type      string `json:"type"`
	Version   string `json:"version"`
	Condition struct {
		BroadcasterUserID string `json:"broadcaster_user_id"`
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

// SubscribeStreamOnline creates a stream.online webhook subscription for the broadcaster.
// Retries once after invalidating the app token cache if Helix returns 401.
// https://dev.twitch.tv/docs/eventsub/eventsub-subscription-types/#streamonline
// https://dev.twitch.tv/docs/api/reference#create-eventsub-subscription
func (c *Client) SubscribeStreamOnline(ctx context.Context, broadcasterUserID string) (SubscribeResult, error) {
	if broadcasterUserID == "" {
		return SubscribeResult{}, errors.New("broadcaster user id is required")
	}

	var payload subscriptionPayload
	payload.Type = "stream.online"
	payload.Version = "1"
	payload.Condition.BroadcasterUserID = broadcasterUserID
	payload.Transport.Method = "webhook"
	payload.Transport.Callback = c.cfg.CallbackURL
	payload.Transport.Secret = c.cfg.EventSubSecret

	body, err := json.Marshal(payload)
	if err != nil {
		return SubscribeResult{}, fmt.Errorf("marshal payload: %w", err)
	}

	var last SubscribeResult
	for attempt := 0; attempt < 2; attempt++ {
		appToken, err := c.AppAccessToken(ctx)
		if err != nil {
			return SubscribeResult{}, fmt.Errorf("app token: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, eventSubSubscriptionsURL, bytes.NewReader(body))
		if err != nil {
			return SubscribeResult{}, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+appToken)
		req.Header.Set("Client-Id", c.cfg.ClientID)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return SubscribeResult{}, fmt.Errorf("request: %w", err)
		}
		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return SubscribeResult{}, fmt.Errorf("read response: %w", readErr)
		}

		last = SubscribeResult{StatusCode: resp.StatusCode, Body: respBody}
		if resp.StatusCode == http.StatusUnauthorized && attempt == 0 {
			c.InvalidateAppAccessToken()
			continue
		}
		return last, nil
	}
	return last, nil
}

func (c *Client) SubscribeStreamOffline(ctx context.Context, broadcasterUserID string) (SubscribeResult, error) {
	if broadcasterUserID == "" {
		return SubscribeResult{}, errors.New("broadcaster user id is required")
	}

	var payload subscriptionPayload
	payload.Type = "stream.offline"
	payload.Version = "1"
	payload.Condition.BroadcasterUserID = broadcasterUserID
	payload.Transport.Method = "webhook"
	payload.Transport.Callback = c.cfg.CallbackURL
	payload.Transport.Secret = c.cfg.EventSubSecret

	body, err := json.Marshal(payload)
	if err != nil {
		return SubscribeResult{}, fmt.Errorf("marshal payload: %w", err)
	}

	var last SubscribeResult
	for attempt := 0; attempt < 2; attempt++ {
		appToken, err := c.AppAccessToken(ctx)
		if err != nil {
			return SubscribeResult{}, fmt.Errorf("app token: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, eventSubSubscriptionsURL, bytes.NewReader(body))
		if err != nil {
			return SubscribeResult{}, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+appToken)
		req.Header.Set("Client-Id", c.cfg.ClientID)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return SubscribeResult{}, fmt.Errorf("request: %w", err)
		}
		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return SubscribeResult{}, fmt.Errorf("read response: %w", readErr)
		}

		last = SubscribeResult{StatusCode: resp.StatusCode, Body: respBody}
		if resp.StatusCode == http.StatusUnauthorized && attempt == 0 {
			c.InvalidateAppAccessToken()
			continue
		}
		return last, nil
	}
	return last, nil
}
