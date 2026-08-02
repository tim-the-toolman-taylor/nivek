// Twitch OAuth (authorization code flow) — the only way users authenticate
// against this system. /start sends them to Twitch with a CSRF state cookie;
// /callback exchanges the code, fetches the streamer's profile, find-or-creates
// a row keyed by twitch_id, then hands the SPA a JWT via URL fragment.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/coreconfig"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/api"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/jwt"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/twitcheventsub"
	userLib "github.com/tim-the-toolman-taylor/nivek/internal/libraries/user"
)

const (
	twitchAuthorizeURL = "https://id.twitch.tv/oauth2/authorize"
	twitchTokenURL     = "https://id.twitch.tv/oauth2/token"
	twitchUsersURL     = "https://api.twitch.tv/helix/users"

	stateCookieName = "twitch_oauth_state"
	stateCookieTTL  = 10 * time.Minute
	httpTimeout     = 10 * time.Second
)

// NewTwitchStartEndpoint kicks off the OAuth dance. We mint a random `state`,
// stash it in a short-lived cookie, and 302 the user to Twitch's authorize URL.
// On the callback we'll require the returned `state` param to match the
// cookie — that's our CSRF defense.
func NewTwitchStartEndpoint(svc nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		cfg, ok := svc.CustomConfig().(coreconfig.CoreApiConfig)
		if !ok {
			return c.String(http.StatusInternalServerError, "twitch oauth not configured")
		}
		if cfg.TwitchClientID == "" || cfg.TwitchRedirectURI == "" {
			return c.String(http.StatusInternalServerError, "twitch oauth not configured")
		}

		state, err := randomURLSafe(24)
		if err != nil {
			svc.Logger().Errorf("twitch oauth: failed to generate state: %s", err.Error())
			return c.String(http.StatusInternalServerError, "internal error")
		}

		c.SetCookie(&http.Cookie{
			Name:     stateCookieName,
			Value:    state,
			Path:     "/api/auth/twitch",
			Expires:  time.Now().Add(stateCookieTTL),
			MaxAge:   int(stateCookieTTL.Seconds()),
			Secure:   true,
			HttpOnly: true,
			// Lax so the cookie comes back on the cross-site GET redirect from
			// Twitch. Strict would drop it. The cookie is HttpOnly + path-scoped
			// to /api/auth/twitch so leakage surface is small.
			SameSite: http.SameSiteLaxMode,
		})

		params := url.Values{}
		params.Set("client_id", cfg.TwitchClientID)
		params.Set("redirect_uri", cfg.TwitchRedirectURI)
		params.Set("response_type", "code")
		// No scope needed — /helix/users returns id/login/display_name with a
		// plain user access token, no extra permissions required.
		params.Set("scope", "")
		params.Set("state", state)

		return c.Redirect(http.StatusFound, twitchAuthorizeURL+"?"+params.Encode())
	}
}

// NewTwitchCallbackEndpoint completes the OAuth exchange and lands the user
// back in the SPA with a JWT. On any failure we redirect to the frontend with
// an `?error=...` query so the SPA can show a useful message instead of a
// bare backend 500.
func NewTwitchCallbackEndpoint(svc nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		cfg, ok := svc.CustomConfig().(coreconfig.CoreApiConfig)
		if !ok {
			return c.String(http.StatusInternalServerError, "twitch oauth not configured")
		}

		fail := func(reason string) error {
			landing := cfg.FrontendBaseURL + "/auth/landing"
			return c.Redirect(http.StatusFound, landing+"?error="+url.QueryEscape(reason))
		}

		// Twitch sends `?error=access_denied` if the user clicks Cancel on the
		// consent screen. Surface that to the SPA instead of treating it as
		// CSRF failure.
		if twErr := c.QueryParam("error"); twErr != "" {
			return fail(twErr)
		}

		code := c.QueryParam("code")
		gotState := c.QueryParam("state")
		if code == "" || gotState == "" {
			return fail("missing_code_or_state")
		}

		stateCookie, err := c.Cookie(stateCookieName)
		if err != nil || stateCookie.Value == "" {
			return fail("missing_state_cookie")
		}
		if stateCookie.Value != gotState {
			return fail("state_mismatch")
		}
		// Burn the cookie so a replay can't reuse it.
		c.SetCookie(&http.Cookie{
			Name:     stateCookieName,
			Value:    "",
			Path:     "/api/auth/twitch",
			MaxAge:   -1,
			Secure:   true,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})

		token, err := exchangeCodeForToken(c.Request().Context(), cfg, code)
		if err != nil {
			svc.Logger().Errorf("twitch oauth: token exchange failed: %s", err.Error())
			return fail("token_exchange_failed")
		}

		profile, err := fetchTwitchProfile(c.Request().Context(), cfg.TwitchClientID, token)
		if err != nil {
			svc.Logger().Errorf("twitch oauth: profile fetch failed: %s", err.Error())
			return fail("profile_fetch_failed")
		}

		userService := userLib.NewService(svc)
		usr, isNew, err := userService.FindOrCreateByTwitchID(userLib.TwitchProfile{
			ID:          profile.ID,
			Login:       profile.Login,
			DisplayName: profile.DisplayName,
		})
		if err != nil {
			svc.Logger().Errorf("twitch oauth: user upsert failed: %s", err.Error())
			return fail("user_upsert_failed")
		}

		if isNew {
			// @TODO::subscribe to webhooks for this new user
			// I want users to eventually opt-in and opt-out of having the bot in chat, regardless of
			// if they have signed up for the website
			// currently signing up for the website is an automatic opt-in, so this logic needs to be
			// de-coupled
			go subscribeToUserWebhooks(context.Background(), cfg, profile.ID, svc.Logger())
		}

		jwtService := jwt.NewJWTService(svc)
		jwtToken, err := jwtService.NewSession(c, usr)
		if err != nil {
			svc.Logger().Errorf("twitch oauth: session issue failed: %s", err.Error())
			return fail("session_failed")
		}

		// URL fragment, not query string: fragments aren't sent to the server
		// and don't end up in access logs / referrer headers. SPA reads it on
		// mount and immediately strips it via history.replaceState.
		landing := cfg.FrontendBaseURL + "/auth/landing#token=" + url.QueryEscape(jwtToken)
		return c.Redirect(http.StatusFound, landing)
	}
}

type twitchTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

func exchangeCodeForToken(ctx context.Context, cfg coreconfig.CoreApiConfig, code string) (string, error) {
	form := url.Values{}
	form.Set("client_id", cfg.TwitchClientID)
	form.Set("client_secret", cfg.TwitchClientSecret)
	form.Set("code", code)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", cfg.TwitchRedirectURI)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, twitchTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("building token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("twitch token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var parsed twitchTokenResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("decoding token response: %w", err)
	}
	if parsed.AccessToken == "" {
		return "", errors.New("twitch token response missing access_token")
	}
	return parsed.AccessToken, nil
}

type twitchUser struct {
	ID          string `json:"id"`
	Login       string `json:"login"`
	DisplayName string `json:"display_name"`
}

type twitchUsersResponse struct {
	Data []twitchUser `json:"data"`
}

func fetchTwitchProfile(ctx context.Context, clientID, accessToken string) (*twitchUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, twitchUsersURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building users request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Client-Id", clientID)

	client := &http.Client{Timeout: httpTimeout}
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

	var parsed twitchUsersResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decoding users response: %w", err)
	}
	if len(parsed.Data) == 0 {
		return nil, errors.New("twitch /helix/users returned empty data array")
	}
	return &parsed.Data[0], nil
}

func randomURLSafe(nBytes int) (string, error) {
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// subscribeToUserWebhooks creates EventSub stream.online webhook subscriptions
// for a newly registered user. Shared Helix client lives in twitcheventsub.
func subscribeToUserWebhooks(ctx context.Context, cfg coreconfig.CoreApiConfig, twitchUserId string, logger *logrus.Logger) {
	client, err := twitcheventsub.NewClient(twitcheventsub.Config{
		ClientID:       cfg.TwitchClientID,
		ClientSecret:   cfg.TwitchClientSecret,
		EventSubSecret: cfg.TwitchEventSubSecret,
		CallbackURL:    fmt.Sprintf("https://peanutbudderbot.com%s", api.TwitchWebhookSubscriptionRequest),
	})
	if err != nil {
		logger.Errorf("failed to subscribe to webhook - client: %s", err.Error())
		return
	}

	result, err := client.SubscribeStreamOnline(ctx, twitchUserId)
	if err != nil {
		logger.Errorf("failed to subscribe to stream.onine webhook: %s", err.Error())
		return
	}

	result, err = client.SubscribeStreamOffline(ctx, twitchUserId)
	if err != nil {
		logger.Errorf("failed to subscribe to stream.offline webhook: %s", err.Error())
		return
	}

	logger.Debugf("webhook subscription response: status [%d] %s", result.StatusCode, string(result.Body))
	if !result.OK() && !result.AlreadyExists() {
		logger.Errorf("failed to subscribe to webhook - unexpected status [%d] %s", result.StatusCode, string(result.Body))
	}
}
