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

// AppTokenSource mints and caches a Twitch app access token (client-credentials
// grant). Safe for concurrent use.
//
// The bot uses this token for the Helix Send Chat Message call: a message sent
// with an APP access token earns the Chat Bot Badge, whereas the same message
// sent with the bot's user token does not. The badge also requires the sender
// (the bot account) to have authorized the app with user:bot + user:write:chat,
// and the broadcaster to have granted channel:bot (or the bot to be a mod) — the
// app token carries no scopes of its own, so those grants live on the accounts.
//
// App tokens are long-lived (~60 days) and carry no refresh token: when one
// expires we simply request another. clientID/clientSecret MUST be the same app
// the bot's other credentials belong to.
type AppTokenSource struct {
	clientID     string
	clientSecret string
	httpClient   *http.Client

	mu          sync.Mutex
	accessToken string
	expiresAt   time.Time
}

func NewAppTokenSource(clientID, clientSecret string) *AppTokenSource {
	return &AppTokenSource{
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   &http.Client{Timeout: 15 * time.Second},
	}
}

// Token returns a valid app access token, minting a new one if the cached token
// is missing or within a one-minute buffer of expiry.
func (s *AppTokenSource) Token(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.accessToken != "" && time.Now().Before(s.expiresAt.Add(-time.Minute)) {
		return s.accessToken, nil
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", s.clientID)
	form.Set("client_secret", s.clientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("building app-token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("app-token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading app-token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		// A 400/401 here means the client_id/client_secret pair is wrong or the
		// app was disabled.
		return "", fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("decoding app-token response: %w", err)
	}
	if parsed.AccessToken == "" {
		return "", fmt.Errorf("token endpoint returned empty access_token")
	}

	s.accessToken = parsed.AccessToken
	ttl := time.Duration(parsed.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = time.Hour // defensive: never cache a non-positive TTL
	}
	s.expiresAt = time.Now().Add(ttl)

	return s.accessToken, nil
}
