// Package twitchauth manages the bot's user access token for Twitch IRC.
//
// Twitch user access tokens (authorization-code grant) expire after a few
// hours. Rather than pasting a new token by hand each time, the bot holds a
// long-lived refresh token and exchanges it for a fresh access token before
// each IRC (re)connect. The IRC session itself survives token expiry once
// connected — the only moment a stale token bites is at connect time — so
// refreshing on connect is sufficient.
package twitchauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const tokenURL = "https://id.twitch.tv/oauth2/token"

// Refresher exchanges a Twitch refresh token for short-lived user access tokens
// and caches the result until just before expiry. Safe for concurrent use.
//
// The refresh token, client ID, and client secret MUST all belong to the same
// registered Twitch application. A refresh token minted by a different app
// (e.g. a third-party token generator using its own client ID) is rejected by
// the token endpoint with a 400/401.
type Refresher struct {
	clientID     string
	clientSecret string
	httpClient   *http.Client

	mu           sync.Mutex
	refreshToken string // rotates — Twitch may return a new one on each refresh
	accessToken  string
	expiresAt    time.Time
}

func NewRefresher(clientID, clientSecret, refreshToken string) *Refresher {
	return &Refresher{
		clientID:     clientID,
		clientSecret: clientSecret,
		refreshToken: refreshToken,
		httpClient:   &http.Client{Timeout: 15 * time.Second},
	}
}

type tokenResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresIn    int      `json:"expires_in"`
	TokenType    string   `json:"token_type"`
	Scope        []string `json:"scope"`
}

// Token returns a valid user access token, refreshing if the cached one is
// missing or within a one-minute buffer of expiry. The returned token has NO
// "oauth:" prefix — an IRC caller must prepend it.
func (r *Refresher) Token(ctx context.Context) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.accessToken != "" && time.Now().Before(r.expiresAt.Add(-time.Minute)) {
		return r.accessToken, nil
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", r.refreshToken)
	form.Set("client_id", r.clientID)
	form.Set("client_secret", r.clientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("building refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("refresh request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading refresh response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		// A 400/401 here usually means the refresh token was revoked (the bot's
		// password changed, or the app was disconnected) or that it was minted
		// by a different client_id than the one sent above.
		return "", fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed tokenResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("decoding refresh response: %w", err)
	}
	if parsed.AccessToken == "" {
		return "", fmt.Errorf("token endpoint returned empty access_token")
	}

	r.accessToken = parsed.AccessToken
	if parsed.RefreshToken != "" {
		// Twitch may rotate the refresh token; keep the newest so the next
		// refresh (this process's lifetime) uses a still-valid one.
		r.refreshToken = parsed.RefreshToken
	}
	ttl := time.Duration(parsed.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = time.Hour // defensive: never cache a non-positive TTL
	}
	r.expiresAt = time.Now().Add(ttl)

	return r.accessToken, nil
}
