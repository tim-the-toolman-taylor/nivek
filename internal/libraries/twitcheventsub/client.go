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
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/api"
)

type TwitchEventSubClient interface {
	SubscribeToAllWebhooks(ctx context.Context, broadcasterTwitchLogin, twitchID string)
	AppAccessToken(ctx context.Context) (string, error)
	InvalidateAppAccessToken()
	SubscribeChannelChatMessages(ctx context.Context, broadcasterUserID string) (SubscribeResult, error)
	SubscribeStreamOnline(ctx context.Context, broadcasterUserID string) (SubscribeResult, error)
	SubscribeStreamOffline(ctx context.Context, broadcasterUserID string) (SubscribeResult, error)
	SubscribeChannelCheer(ctx context.Context, broadcasterUserID string) (SubscribeResult, error)
	SubscribeChannelPointsRedemptionAdd(ctx context.Context, broadcasterUserID string) (SubscribeResult, error)
	SubscribeCustomPowerUpRedemptionAdd(ctx context.Context, broadcasterUserID string) (SubscribeResult, error)
	ListEventSubSubscriptions(ctx context.Context) ([]EventSubSubscription, error)
	DeleteEventSubSubscription(ctx context.Context, id string) error
	doAppGet(ctx context.Context, reqURL string) ([]byte, int, error)
	attemptNewSubscription(ctx context.Context, payload subscriptionPayload) (SubscribeResult, error)
	FetchTwitchProfile(ctx context.Context, broadcasterUserId, accessToken *string) (*TwitchUser, error)
	fetchTwitchProfileByBroadcasterId(ctx context.Context, broadcasterUserId string) (*TwitchUser, error)
	fetchTwitchProfileByAccessToken(ctx context.Context, accessToken string) (*TwitchUser, error)
	IsStreamLive(ctx context.Context, broadcasterUserID string) (bool, error)
}

// Client mints app tokens and creates EventSub webhook subscriptions.
type clientImpl struct {
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
func NewClient(cfg Config) (TwitchEventSubClient, error) {
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
	return &clientImpl{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

func (c *clientImpl) SubscribeToAllWebhooks(ctx context.Context, broadcasterTwitchLogin, twitchID string) {
	webhooks := map[string]func(context.Context, string) (SubscribeResult, error){
		"stream.online":        c.SubscribeStreamOnline,
		"stream.offline":       c.SubscribeStreamOffline,
		"channel.chat.message": c.SubscribeChannelChatMessages,
	}

	var ok, exists, failed int
	for webhookName, webhookFunc := range webhooks {
		result, err := webhookFunc(ctx, twitchID)
		if err != nil {
			failed++
			log.Printf("FAIL %s broadcaster=%s twitch_id=%s err=%v",
				webhookName, broadcasterTwitchLogin, twitchID, err)
		} else if result.AlreadyExists() {
			exists++
			log.Printf("%s already-subscribed username=%s twitch_id=%s",
				webhookName, broadcasterTwitchLogin, twitchID)
		} else if result.OK() {
			ok++
			log.Printf("subscribed username=%s twitch_id=%s status=%d",
				broadcasterTwitchLogin, twitchID, result.StatusCode)
		} else {
			failed++
			log.Printf("FAIL username=%s twitch_id=%s status=%d body=%s",
				broadcasterTwitchLogin, twitchID, result.StatusCode, string(result.Body))
		}
	}
}

func (c *clientImpl) attemptNewSubscription(ctx context.Context, payload subscriptionPayload) (SubscribeResult, error) {
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

// InvalidateAppAccessToken drops the cache (e.g. after Helix 401).
func (c *clientImpl) InvalidateAppAccessToken() {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	c.token = ""
	c.tokenExpiry = time.Time{}
}

func (c *clientImpl) SubscribeChannelChatMessages(ctx context.Context, broadcasterUserID string) (SubscribeResult, error) {
	if broadcasterUserID == "" {
		return SubscribeResult{}, errors.New("broadcaster user id is required")
	}

	botId := "1322716097"
	var payload subscriptionPayload
	payload.Type = "channel.chat.message"
	payload.Version = "1"
	payload.Condition.BroadcasterUserID = broadcasterUserID
	payload.Condition.UserId = &botId
	payload.Transport.Method = "webhook"
	payload.Transport.Callback = c.cfg.CallbackURL
	payload.Transport.Secret = c.cfg.EventSubSecret

	return c.attemptNewSubscription(ctx, payload)
}

// SubscribeStreamOnline creates a stream.online webhook subscription for the broadcaster.
// Retries once after invalidating the app token cache if Helix returns 401.
// https://dev.twitch.tv/docs/eventsub/eventsub-subscription-types/#streamonline
// https://dev.twitch.tv/docs/api/reference#create-eventsub-subscription
func (c *clientImpl) SubscribeStreamOnline(ctx context.Context, broadcasterUserID string) (SubscribeResult, error) {
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

func (c *clientImpl) SubscribeStreamOffline(ctx context.Context, broadcasterUserID string) (SubscribeResult, error) {
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

// SubscribeCustomPowerUpRedemptionAdd creates a
// channel.custom_power_up_redemption.add webhook subscription for the
// broadcaster. Custom Power-ups are paid with Bits; used by the overlay relay
// (construct this client with the overlay callback + secret). The type string
// mirrors overlayrelay.SubTypePowerUp; keep them in sync. Requires the
// broadcaster's bits:read grant (already requested at login). Open beta as of 2026.
// https://dev.twitch.tv/docs/eventsub/eventsub-subscription-types/#channelcustom_power_up_redemptionadd
func (c *clientImpl) SubscribeCustomPowerUpRedemptionAdd(ctx context.Context, broadcasterUserID string) (SubscribeResult, error) {
	if broadcasterUserID == "" {
		return SubscribeResult{}, errors.New("broadcaster user id is required")
	}

	var payload subscriptionPayload
	payload.Type = "channel.custom_power_up_redemption.add"
	payload.Version = "1"
	payload.Condition.BroadcasterUserID = broadcasterUserID
	payload.Transport.Method = "webhook"
	payload.Transport.Callback = c.cfg.CallbackURL
	payload.Transport.Secret = c.cfg.EventSubSecret

	return c.attemptNewSubscription(ctx, payload)
}

// SubscribeChannelCheer creates a channel.cheer webhook subscription for the
// broadcaster. Used by the overlay relay (construct this client with the
// overlay callback + secret). The type string mirrors
// overlayrelay.SubTypeCheer; keep them in sync.
// https://dev.twitch.tv/docs/eventsub/eventsub-subscription-types/#channelcheer
func (c *clientImpl) SubscribeChannelCheer(ctx context.Context, broadcasterUserID string) (SubscribeResult, error) {
	if broadcasterUserID == "" {
		return SubscribeResult{}, errors.New("broadcaster user id is required")
	}

	var payload subscriptionPayload
	payload.Type = "channel.cheer"
	payload.Version = "1"
	payload.Condition.BroadcasterUserID = broadcasterUserID
	payload.Transport.Method = "webhook"
	payload.Transport.Callback = c.cfg.CallbackURL
	payload.Transport.Secret = c.cfg.EventSubSecret

	return c.attemptNewSubscription(ctx, payload)
}

// SubscribeChannelPointsRedemptionAdd creates a
// channel.channel_points_custom_reward_redemption.add webhook subscription for
// the broadcaster (all rewards; reward_id is intentionally omitted). Used by the
// overlay relay. The type string mirrors overlayrelay.SubTypeRedemption; keep
// them in sync.
// https://dev.twitch.tv/docs/eventsub/eventsub-subscription-types/#channelchannel_points_custom_reward_redemptionadd
func (c *clientImpl) SubscribeChannelPointsRedemptionAdd(ctx context.Context, broadcasterUserID string) (SubscribeResult, error) {
	if broadcasterUserID == "" {
		return SubscribeResult{}, errors.New("broadcaster user id is required")
	}

	var payload subscriptionPayload
	payload.Type = "channel.channel_points_custom_reward_redemption.add"
	payload.Version = "1"
	payload.Condition.BroadcasterUserID = broadcasterUserID
	payload.Transport.Method = "webhook"
	payload.Transport.Callback = c.cfg.CallbackURL
	payload.Transport.Secret = c.cfg.EventSubSecret

	return c.attemptNewSubscription(ctx, payload)
}

// ListEventSubSubscriptions returns every EventSub subscription registered for
// the app, across all pages and regardless of status. Used to audit webhook
// health. Retries once after invalidating the app token cache on Helix 401.
// https://dev.twitch.tv/docs/api/reference/#get-eventsub-subscriptions
func (c *clientImpl) ListEventSubSubscriptions(ctx context.Context) ([]EventSubSubscription, error) {
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
func (c *clientImpl) DeleteEventSubSubscription(ctx context.Context, id string) error {
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
func (c *clientImpl) doAppGet(ctx context.Context, reqURL string) ([]byte, int, error) {
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

func (c *clientImpl) FetchTwitchProfile(
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

func (c *clientImpl) fetchTwitchProfileByBroadcasterId(
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

func (c *clientImpl) fetchTwitchProfileByAccessToken(
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

func (c *clientImpl) IsStreamLive(ctx context.Context, broadcasterUserID string) (bool, error) {
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
