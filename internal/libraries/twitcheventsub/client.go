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
	TwitchUsersURL           = "https://api.twitch.tv/helix/users"
	defaultHTTPTimeout       = 10 * time.Second
	// Refresh a minute early so we don't race the exact expiry second.
	appTokenExpirySkew = time.Minute
	HttpTimeout        = 10 * time.Second
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

type TwitchUser struct {
	ID          string `json:"id"`
	Login       string `json:"login"`
	DisplayName string `json:"display_name"`
}

type TwitchUsersResponse struct {
	Data []TwitchUser `json:"data"`
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

func (c *Client) FetchTwitchProfile(
	ctx context.Context,
	broadcasterUserId,
	accessToken *string,
) (
	*TwitchUser,
	error,
) {
	if accessToken != nil {
		return c.fetchTwitchProfileByAccessToken(
			ctx,
			*accessToken,
		)
	}

	if broadcasterUserId != nil {
		return c.fetchTwitchProfileByBroadcasterId(
			ctx,
			*broadcasterUserId,
		)
	}

	return nil, fmt.Errorf(
		"missing required parameter - either broadcasterUserId or accessToken must be present",
	)
}

func (c *Client) fetchTwitchProfileByBroadcasterId(
	ctx context.Context,
	broadcasterUserId string,
) (
	*TwitchUser,
	error,
) {
	accessToken, err := c.AppAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("app token: %w", err)
	}

	u, err := url.Parse(TwitchUsersURL)
	if err != nil {
		return nil, fmt.Errorf(
			"error parsing twitch-users-url %s - %s",
			TwitchUsersURL,
			err.Error(),
		)
	}

	q := u.Query()
	q.Set("id", broadcasterUserId)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("building users request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Client-Id", c.cfg.ClientID)

	client := &http.Client{Timeout: HttpTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("users request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading users response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("twitch /helix/users returned %d: %s", resp.StatusCode, string(body))
	}

	var parsed TwitchUsersResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decoding users response: %w", err)
	}
	if len(parsed.Data) == 0 {
		return nil, errors.New("twitch /helix/users returned empty data array")
	}
	return &parsed.Data[0], nil
}

func (c *Client) fetchTwitchProfileByAccessToken(
	ctx context.Context,
	accessToken string,
) (
	*TwitchUser,
	error,
) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, TwitchUsersURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building users request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Client-Id", c.cfg.ClientID)

	client := &http.Client{Timeout: HttpTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("users request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading users response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("twitch /helix/users returned %d: %s", resp.StatusCode, string(body))
	}

	var parsed TwitchUsersResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decoding users response: %w", err)
	}
	if len(parsed.Data) == 0 {
		return nil, errors.New("twitch /helix/users returned empty data array")
	}
	return &parsed.Data[0], nil
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

func (c *Client) SubscribeChannelChatMessages(ctx context.Context, broadcasterUserID string) (SubscribeResult, error) {
	if broadcasterUserID == "" {
		return SubscribeResult{}, errors.New("broadcaster user id is required")
	}

	var payload subscriptionPayload
	payload.Type = "channel.chat.message"
	payload.Version = "1"
	payload.Condition.BroadcasterUserID = broadcasterUserID
	payload.Transport.Method = "webhook"
	payload.Transport.Callback = c.cfg.CallbackURL
	payload.Transport.Secret = c.cfg.EventSubSecret

	return c.attemptNewSubscription(ctx, payload)
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

	return c.attemptNewSubscription(ctx, payload)
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

	return c.attemptNewSubscription(ctx, payload)
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

// StatusEnabled is the only healthy EventSub subscription status; any other
// value (webhook_callback_verification_failed, notification_failures_exceeded,
// authorization_revoked, …) means Twitch will not deliver notifications.
const StatusEnabled = "enabled"

// ListEventSubSubscriptions returns every EventSub subscription registered for
// the app, across all pages and regardless of status. Used to audit webhook
// health. Retries once after invalidating the app token cache on Helix 401.
// https://dev.twitch.tv/docs/api/reference/#get-eventsub-subscriptions
func (c *Client) ListEventSubSubscriptions(ctx context.Context) ([]EventSubSubscription, error) {
	var out []EventSubSubscription
	cursor := ""
	for {
		reqURL := eventSubSubscriptionsURL
		if cursor != "" {
			q := url.Values{}
			q.Set("after", cursor)
			reqURL += "?" + q.Encode()
		}

		body, status, err := c.doAppGet(ctx, reqURL)
		if err != nil {
			return nil, err
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("list eventsub subscriptions returned %d: %s", status, string(body))
		}

		var page struct {
			Data       []EventSubSubscription `json:"data"`
			Pagination struct {
				Cursor string `json:"cursor"`
			} `json:"pagination"`
		}
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("decode subscriptions page: %w", err)
		}
		out = append(out, page.Data...)
		if page.Pagination.Cursor == "" {
			break
		}
		cursor = page.Pagination.Cursor
	}
	return out, nil
}

// DeleteEventSubSubscription removes a subscription by id. 404 (already gone) is
// treated as success so callers can converge idempotently. Retries once after
// invalidating the app token cache on Helix 401.
// https://dev.twitch.tv/docs/api/reference/#delete-eventsub-subscription
func (c *Client) DeleteEventSubSubscription(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("subscription id is required")
	}
	q := url.Values{}
	q.Set("id", id)
	reqURL := eventSubSubscriptionsURL + "?" + q.Encode()

	for attempt := 0; attempt < 2; attempt++ {
		appToken, err := c.AppAccessToken(ctx)
		if err != nil {
			return fmt.Errorf("app token: %w", err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, reqURL, nil)
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+appToken)
		req.Header.Set("Client-Id", c.cfg.ClientID)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("request: %w", err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusUnauthorized && attempt == 0 {
			c.InvalidateAppAccessToken()
			continue
		}
		if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
			return nil
		}
		return fmt.Errorf("delete subscription %s returned %d: %s", id, resp.StatusCode, string(body))
	}
	return nil
}

// doAppGet performs a GET authenticated with the app token, retrying once on
// 401 after invalidating the cached token. Returns the raw body and status.
func (c *Client) doAppGet(ctx context.Context, reqURL string) ([]byte, int, error) {
	var lastBody []byte
	var lastStatus int
	for attempt := 0; attempt < 2; attempt++ {
		appToken, err := c.AppAccessToken(ctx)
		if err != nil {
			return nil, 0, fmt.Errorf("app token: %w", err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, 0, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+appToken)
		req.Header.Set("Client-Id", c.cfg.ClientID)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, 0, fmt.Errorf("request: %w", err)
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, 0, fmt.Errorf("read response: %w", readErr)
		}
		lastBody, lastStatus = body, resp.StatusCode
		if resp.StatusCode == http.StatusUnauthorized && attempt == 0 {
			c.InvalidateAppAccessToken()
			continue
		}
		return body, resp.StatusCode, nil
	}
	return lastBody, lastStatus, nil
}

func (c *Client) attemptNewSubscription(ctx context.Context, payload subscriptionPayload) (SubscribeResult, error) {
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
