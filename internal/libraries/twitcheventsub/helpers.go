package twitcheventsub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// AppAccessToken returns a cached app access token, minting one if needed.
// https://dev.twitch.tv/docs/authentication/getting-tokens-oauth/#client-credentials-grant-flow
func (c *clientImpl) AppAccessToken(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	if c.token != "" && time.Now().Before(c.tokenExpiry.Add(-appTokenExpirySkew)) {
		return c.token, nil
	}

	token, expiresIn, err := fetchAppAccessToken(
		ctx,
		c.cfg.ClientID,
		c.cfg.ClientSecret,
		c.httpClient,
	)
	if err != nil {
		return "", err
	}
	c.token = token
	c.tokenExpiry = time.Now().Add(time.Duration(expiresIn) * time.Second)
	return c.token, nil
}

func fetchAppAccessToken(
	ctx context.Context,
	clientId,
	clientSecret string,
	httpClient *http.Client,
) (
	token string,
	expiresIn int,
	err error,
) {
	form := url.Values{}
	form.Set("client_id", clientId)
	form.Set("client_secret", clientSecret)
	form.Set("grant_type", "client_credentials")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, fmt.Errorf("building app token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
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
