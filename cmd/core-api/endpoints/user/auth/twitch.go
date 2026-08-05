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
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/botclient"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/jwt"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/twitcheventsub"
	userLib "github.com/tim-the-toolman-taylor/nivek/internal/libraries/user"
)

const (
	twitchAuthorizeURL = "https://id.twitch.tv/oauth2/authorize"
	twitchTokenURL     = "https://id.twitch.tv/oauth2/token"

	stateCookieName = "twitch_oauth_state"
	stateCookieTTL  = 10 * time.Minute
)

type twitchTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

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

		esClient, err := twitcheventsub.NewClient(twitcheventsub.Config{
			ClientID:       cfg.TwitchClientID,
			ClientSecret:   cfg.TwitchClientSecret,
			EventSubSecret: cfg.TwitchEventSubSecret,
		})
		if err != nil {
			svc.Logger().Errorf("join-if-live: eventsub client: %s", err.Error())
			return c.JSON(
				http.StatusInternalServerError,
				map[string]string{
					"error": "failed to create event-sub client",
				},
			)
		}

		// call twitch /helix/users with a "who owns this token" to fetch user profile
		// we don't have their twitch_login or broadcaster_user_id, so this is the necessary flow
		profile, err := esClient.FetchTwitchProfile(
			c.Request().Context(),
			nil,
			&token,
		)
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
			// @TODO::opt-in and opt-out of having the bot in chat. Currently it is only opt-in
			go subscribeToUserWebhooks(context.Background(), cfg, profile.ID, svc.Logger())
			go joinBotIfLive(context.Background(), esClient, cfg, userService, profile, svc.Logger())
		}

		if !isNew {
			// A legacy (pre-OAuth) user just authenticated, or a returning user
			// logged in. Tell the bot to stop the hourly "please authenticate"
			// nag loop for their channel. Skipped for brand-new signups (isNew),
			// which never had a legacy nag loop; harmless no-op for returning
			// non-legacy users.
			go stopBotherLoop(cfg, profile.Login, svc.Logger())
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

func exchangeCodeForToken(
	ctx context.Context,
	cfg coreconfig.CoreApiConfig,
	code string,
) (
	string,
	error,
) {
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

	client := &http.Client{Timeout: twitcheventsub.HttpTimeout}
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

func randomURLSafe(nBytes int) (string, error) {
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// joinBotIfLive checks whether a newly-registered streamer is live right now
// and, if so, tells the bot to join their chat immediately. EventSub only fires
// on the live *transition*, so a user already streaming at signup would be
// missed until their next go-live or a bot restart. It also flips is_live in
// the DB so a bot restart re-joins them; the join is the user-facing win, so DB
// state is best-effort.
func joinBotIfLive(
	ctx context.Context,
	esClient *twitcheventsub.Client,
	cfg coreconfig.CoreApiConfig,
	userService userLib.NivekUserService,
	profile *twitcheventsub.TwitchUser,
	logger *logrus.Logger,
) {
	live, err := esClient.IsStreamLive(ctx, profile.ID)
	if err != nil {
		logger.Errorf("join-if-live: stream status check for %s: %s", profile.Login, err.Error())
		return
	}
	if !live {
		return
	}

	if err := userService.PutChannelState(profile.Login, true); err != nil {
		logger.Errorf("join-if-live: failed to set is_live for %s: %s", profile.Login, err.Error())
		// keep going — the join below is the point; DB state is best-effort.
	}

	botClient, err := botclient.NewClient(cfg.BotInternalURL, cfg.BotAPIHMACKey)
	if err != nil {
		logger.Errorf("join-if-live: bot client: %s", err.Error())
		return
	}
	if err := botClient.JoinChannel(profile.Login); err != nil {
		logger.Errorf("join-if-live: failed to push join for %s: %s", profile.Login, err.Error())
		return
	}
	logger.Infof("join-if-live: bot joining %s (live at signup)", profile.Login)
}

// stopBotherLoop tells the bot to end the hourly "please authenticate" nag loop
// for a channel now that its owner has authenticated. Best-effort: the bot
// treats an unknown channel as a no-op, and a bot restart already excludes
// authenticated users, so a failed push is not fatal.
func stopBotherLoop(cfg coreconfig.CoreApiConfig, login string, logger *logrus.Logger) {
	botClient, err := botclient.NewClient(cfg.BotInternalURL, cfg.BotAPIHMACKey)
	if err != nil {
		logger.Errorf("stop-bother: bot client: %s", err.Error())
		return
	}
	if err := botClient.StopBother(login); err != nil {
		logger.Errorf("stop-bother: push for %s: %s", login, err.Error())
	}
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
