// Package auth implements the Twitch OAuth authorization-code flow used for
// sign-in. OAuth state is kept in a short-lived HttpOnly cookie, the provider
// access token is used only to resolve the Twitch identity, and the local
// application session is returned as an HttpOnly cookie rather than exposing a
// JWT to browser JavaScript.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
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
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/alerting"
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
	stateCookiePath = "/api/auth/twitch/callback"
	stateCookieTTL  = 10 * time.Minute
	providerTimeout = 12 * time.Second
	backgroundTTL   = 20 * time.Second
	maxProviderBody = 64 << 10
)

type twitchTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type twitchErrorResponse struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
	Error   string `json:"error"`
}

type twitchUsersResponse struct {
	Data []twitcheventsub.TwitchUser `json:"data"`
}

func NewTwitchStartEndpoint(svc nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		cfg, ok := svc.CustomConfig().(coreconfig.CoreAPIConfig)
		if !ok {
			return echo.NewHTTPError(http.StatusInternalServerError, "oauth configuration unavailable")
		}

		setNoStoreHeaders(c.Response().Header())
		state, err := randomURLSafe(32)
		if err != nil {
			svc.Logger().Errorf("twitch oauth: generate state: %s", err.Error())
			return echo.NewHTTPError(http.StatusInternalServerError, "unable to start sign-in")
		}

		c.SetCookie(&http.Cookie{
			Name:     stateCookieName,
			Value:    state,
			Path:     stateCookiePath,
			Expires:  time.Now().UTC().Add(stateCookieTTL),
			MaxAge:   int(stateCookieTTL.Seconds()),
			Secure:   cfg.SessionCookieSecure,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})

		params := url.Values{}
		params.Set("client_id", cfg.TwitchClientID)
		params.Set("redirect_uri", cfg.TwitchRedirectURI)
		params.Set("response_type", "code")
		params.Set("state", state)
		// channel:bot lets the app read the broadcaster's chat via EventSub
		// (channel.chat.message) without the bot having to be a moderator.
		params.Set("scope", "channel:bot")

		return c.Redirect(http.StatusFound, twitchAuthorizeURL+"?"+params.Encode())
	}
}

func NewTwitchCallbackEndpoint(svc nivek.NivekService) echo.HandlerFunc {
	loginNotifier := alerting.NewLoginNotifier()
	return func(c echo.Context) error {
		cfg, ok := svc.CustomConfig().(coreconfig.CoreAPIConfig)
		if !ok {
			return echo.NewHTTPError(http.StatusInternalServerError, "oauth configuration unavailable")
		}

		setNoStoreHeaders(c.Response().Header())
		fail := func(reason string) error {
			clearOAuthStateCookie(c, cfg)
			jwt.NewJWTService(svc).ClearSession(c)
			return redirectToLanding(c, cfg, reason)
		}

		if providerError := c.QueryParam("error"); providerError != "" {
			if providerError == "access_denied" {
				return fail("access_denied")
			}
			svc.Logger().Warnf("twitch oauth: provider returned %q", providerError)
			return fail("provider_error")
		}

		code := strings.TrimSpace(c.QueryParam("code"))
		gotState := strings.TrimSpace(c.QueryParam("state"))
		if code == "" || gotState == "" {
			return fail("missing_code_or_state")
		}

		stateCookie, err := c.Cookie(stateCookieName)
		if err != nil || stateCookie.Value == "" {
			return fail("missing_state_cookie")
		}
		clearOAuthStateCookie(c, cfg)
		if len(stateCookie.Value) != len(gotState) || subtle.ConstantTimeCompare([]byte(stateCookie.Value), []byte(gotState)) != 1 {
			return fail("state_mismatch")
		}

		ctx, cancel := context.WithTimeout(c.Request().Context(), providerTimeout)
		defer cancel()

		accessToken, err := exchangeCodeForToken(ctx, cfg, code)
		if err != nil {
			svc.Logger().Warnf("twitch oauth: token exchange failed: %s", err.Error())
			return fail("token_exchange_failed")
		}

		profile, err := fetchTwitchProfile(ctx, cfg.TwitchClientID, accessToken)
		if err != nil {
			svc.Logger().Warnf("twitch oauth: profile fetch failed: %s", err.Error())
			return fail("profile_fetch_failed")
		}
		if profile == nil || profile.ID == "" || profile.Login == "" {
			return fail("invalid_profile")
		}

		userService := userLib.NewService(svc)
		usr, isNew, err := userService.FindOrCreateByTwitchIDAndTwitchLogin(userLib.TwitchProfile{
			ID:          profile.ID,
			Login:       profile.Login,
			DisplayName: profile.DisplayName,
		})
		if err != nil {
			svc.Logger().Errorf("twitch oauth: user upsert failed: %s", err.Error())
			return fail("user_upsert_failed")
		}

		if err := jwt.NewJWTService(svc).NewSession(c, usr); err != nil {
			svc.Logger().Errorf("twitch oauth: session issue failed: %s", err.Error())
			return fail("session_failed")
		}

		// The success path was previously silent, so a broken signup funnel had
		// no positive signal to miss. Log every sign-in with the new-vs-returning
		// bit — this is the counterpart the "signups flatlined" check reads.
		svc.Logger().WithFields(logrus.Fields{
			"twitch_login": profile.Login,
			"twitch_id":    profile.ID,
			"is_new":       isNew,
		}).Info("twitch oauth: sign-in ok")
		// Friendly Discord ping on every sign-in (reuses CORE_API_ALERT_WEBHOOK;
		// no-op if unset). Off the response path so a slow Discord can't stall login.
		go loginNotifier.NotifyLogin(profile.Login, isNew)

		if isNew {
			go runBackground(svc.Logger(), "eventsub subscription", func(ctx context.Context) {
				subscribeToUserWebhooks(ctx, cfg, profile.ID, svc.Logger())
			})
			go runBackground(svc.Logger(), "join live channel", func(ctx context.Context) {
				esClient, clientErr := twitcheventsub.NewClient(twitcheventsub.Config{
					ClientID:       cfg.TwitchClientID,
					ClientSecret:   cfg.TwitchClientSecret,
					EventSubSecret: cfg.TwitchEventSubSecret,
				})
				if clientErr != nil {
					svc.Logger().Warnf("join-if-live skipped: %s", clientErr.Error())
					return
				}
				joinBotIfLive(ctx, esClient, cfg, userService, profile, svc.Logger())
			})
		}

		return redirectToLanding(c, cfg, "")
	}
}

func exchangeCodeForToken(ctx context.Context, cfg coreconfig.CoreAPIConfig, code string) (string, error) {
	form := url.Values{}
	form.Set("client_id", cfg.TwitchClientID)
	form.Set("client_secret", cfg.TwitchClientSecret)
	form.Set("code", code)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", cfg.TwitchRedirectURI)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, twitchTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	req.Header.Set(echo.HeaderAccept, echo.MIMEApplicationJSON)

	client := &http.Client{Timeout: providerTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxProviderBody))
	if err != nil {
		return "", fmt.Errorf("read token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		var providerErr twitchErrorResponse
		_ = json.Unmarshal(body, &providerErr)
		code := providerErr.Error
		if code == "" {
			code = "provider_rejected_request"
		}
		return "", fmt.Errorf("Twitch token endpoint returned %d (%s)", resp.StatusCode, code)
	}

	var parsed twitchTokenResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	if parsed.AccessToken == "" {
		return "", errors.New("token response missing access_token")
	}
	if parsed.TokenType != "" && !strings.EqualFold(parsed.TokenType, "bearer") {
		return "", errors.New("token response contained an unexpected token type")
	}
	return parsed.AccessToken, nil
}

func fetchTwitchProfile(ctx context.Context, clientID, accessToken string) (*twitcheventsub.TwitchUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, twitcheventsub.TwitchUsersURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build profile request: %w", err)
	}
	req.Header.Set("Client-Id", clientID)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+accessToken)
	req.Header.Set(echo.HeaderAccept, echo.MIMEApplicationJSON)

	resp, err := (&http.Client{Timeout: providerTimeout}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("profile request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxProviderBody))
	if err != nil {
		return nil, fmt.Errorf("read profile response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Twitch profile endpoint returned %d", resp.StatusCode)
	}

	var parsed twitchUsersResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode profile response: %w", err)
	}
	if len(parsed.Data) != 1 {
		return nil, fmt.Errorf("profile response contained %d users", len(parsed.Data))
	}
	return &parsed.Data[0], nil
}

func redirectToLanding(c echo.Context, cfg coreconfig.CoreAPIConfig, reason string) error {
	landing, err := url.Parse(cfg.FrontendBaseURL + "/auth/landing")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "invalid frontend configuration")
	}
	if reason != "" {
		query := landing.Query()
		query.Set("error", reason)
		landing.RawQuery = query.Encode()
	}
	return c.Redirect(http.StatusSeeOther, landing.String())
}

func clearOAuthStateCookie(c echo.Context, cfg coreconfig.CoreAPIConfig) {
	c.SetCookie(&http.Cookie{
		Name:     stateCookieName,
		Value:    "",
		Path:     stateCookiePath,
		Expires:  time.Unix(1, 0).UTC(),
		MaxAge:   -1,
		Secure:   cfg.SessionCookieSecure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func setNoStoreHeaders(header http.Header) {
	header.Set(echo.HeaderCacheControl, "no-store")
	header.Set("Pragma", "no-cache")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("X-Content-Type-Options", "nosniff")
}

func randomURLSafe(nBytes int) (string, error) {
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func runBackground(logger *logrus.Logger, name string, fn func(context.Context)) {
	ctx, cancel := context.WithTimeout(context.Background(), backgroundTTL)
	defer cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			logger.Errorf("%s panic: %v", name, recovered)
		}
	}()
	fn(ctx)
}

func joinBotIfLive(
	ctx context.Context,
	esClient *twitcheventsub.Client,
	cfg coreconfig.CoreAPIConfig,
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
		logger.Errorf("join-if-live: set is_live for %s: %s", profile.Login, err.Error())
	}

	botClient, err := botclient.NewClient(cfg.BotInternalURL, cfg.BotAPIHMACKey)
	if err != nil {
		logger.Errorf("join-if-live: bot client: %s", err.Error())
		return
	}
	if err := botClient.JoinChannel(profile.Login); err != nil {
		logger.Errorf("join-if-live: join %s: %s", profile.Login, err.Error())
		return
	}
	logger.Infof("join-if-live: bot joining %s", profile.Login)
}

func subscribeToUserWebhooks(ctx context.Context, cfg coreconfig.CoreAPIConfig, twitchUserID string, logger *logrus.Logger) {
	if cfg.TwitchEventSubCallbackURL == "" {
		logger.Warn("eventsub subscription skipped: TWITCH_EVENTSUB_CALLBACK_URL is not configured")
		return
	}

	client, err := twitcheventsub.NewClient(twitcheventsub.Config{
		ClientID:       cfg.TwitchClientID,
		ClientSecret:   cfg.TwitchClientSecret,
		EventSubSecret: cfg.TwitchEventSubSecret,
		CallbackURL:    cfg.TwitchEventSubCallbackURL,
	})
	if err != nil {
		logger.Errorf("eventsub: create client: %s", err.Error())
		return
	}

	result, err := client.SubscribeStreamOnline(ctx, twitchUserID)
	if err != nil {
		logger.Errorf("eventsub: subscribe stream.online: %s", err.Error())
		return
	}
	if !result.OK() && !result.AlreadyExists() {
		logger.Errorf("eventsub: stream.online returned status %d", result.StatusCode)
		return
	}

	result, err = client.SubscribeStreamOffline(ctx, twitchUserID)
	if err != nil {
		logger.Errorf("eventsub: subscribe stream.offline: %s", err.Error())
		return
	}
	if !result.OK() && !result.AlreadyExists() {
		logger.Errorf("eventsub: stream.offline returned status %d", result.StatusCode)
	}
}
