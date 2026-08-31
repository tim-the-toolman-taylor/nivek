package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/sirupsen/logrus"
	"github.com/sourcegraph/conc/pool"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/coreconfig"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/endpoints/overlay"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/routes"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/alerting"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/jwt"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/overlayrelay"
)

func main() {
	cfg := coreconfig.GetCoreAPIConfig()
	if err := cfg.Validate(); err != nil {
		panic(err)
	}
	if err := jwt.ValidateJWTSecret(); err != nil {
		panic(err)
	}

	nivek.Bootstrap(
		nivek.BootstrapParameters{
			NivekServiceConfig: nivek.NivekServiceConfig{
				UsePSQL:                    true,
				RequireStartupConnections:  true,
				StartupConnectionsPostgres: nivek.GetStartupConnectionsForPostgres(),
			},
			CustomConfig: cfg,
		},
		func(svc nivek.NivekService, ctx context.Context) error {
			// Page a human on any Error/Fatal/Panic log. Inert if the webhook
			// env var is unset, so non-prod envs stay quiet.
			if hook := alerting.NewDiscordErrorHook(); hook.Enabled() {
				svc.Logger().AddHook(hook)
				svc.Logger().Info("discord error alerting enabled")
			} else {
				svc.Logger().Infof("discord error alerting disabled (%s unset)", alerting.AlertWebhookEnv)
			}

			e := echo.New()
			e.HideBanner = true
			e.HidePort = true
			e.Server.ReadHeaderTimeout = 5 * time.Second
			e.Server.ReadTimeout = 20 * time.Second
			e.Server.WriteTimeout = 30 * time.Second
			e.Server.IdleTimeout = 60 * time.Second

			e.Use(middleware.Recover())
			e.Use(middleware.RequestID())
			// Structured access log routed through logrus. 5xx (and handler
			// errors) log at Error level, which the Discord hook picks up; the
			// message is kept low-cardinality (method + route pattern) so
			// repeated failures of one route dedupe into a single alert. Skips
			// the health check to avoid flooding the log.
			e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
				Skipper: func(c echo.Context) bool {
					return c.Request().URL.Path == "/api/healthz"
				},
				LogStatus:    true,
				LogMethod:    true,
				LogURI:       true,
				LogLatency:   true,
				LogRequestID: true,
				LogError:     true,
				HandleError:  true,
				LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
					entry := svc.Logger().WithFields(logrus.Fields{
						"method":     v.Method,
						"route":      c.Path(),
						"uri":        v.URI,
						"status":     v.Status,
						"latency_ms": v.Latency.Milliseconds(),
						"request_id": v.RequestID,
					})
					// Severity keys off the resolved STATUS, not the mere
					// presence of v.Error: HandleError sets v.Error for ordinary
					// 4xx client errors too (e.g. a logged-out 401 on
					// /api/profile), and those must not log at Error level or
					// fire the Discord alert hook.
					if v.Error != nil {
						entry = entry.WithField("error", v.Error.Error())
					}
					switch {
					case v.Status >= 500:
						entry.Errorf("5xx %s %s", v.Method, c.Path())
					case v.Status >= 400:
						entry.Warnf("4xx %s %s", v.Method, c.Path())
					default:
						entry.Infof("%s %s", v.Method, c.Path())
					}
					return nil
				},
			}))
			e.Use(middleware.BodyLimit("1M"))
			// A WebSocket 101 handshake must stay a bare handshake. Gzip and Secure
			// decorate every response with headers (Vary, CSP, X-Frame-Options, ...);
			// on a 101 those extra headers stop Traefik from recognising the upgrade
			// and tunnelling it, so the socket dies with a 502 after the hello
			// timeout. Skip both for upgrade requests -- they only matter for normal
			// HTTP responses anyway. The overlay connect endpoint is the one WS route.
			isWebSocketUpgrade := func(c echo.Context) bool {
				return strings.EqualFold(c.Request().Header.Get(echo.HeaderUpgrade), "websocket")
			}
			e.Use(middleware.GzipWithConfig(middleware.GzipConfig{Skipper: isWebSocketUpgrade}))
			e.Use(middleware.SecureWithConfig(middleware.SecureConfig{
				Skipper:               isWebSocketUpgrade,
				XSSProtection:         "0",
				ContentTypeNosniff:    "nosniff",
				XFrameOptions:         "DENY",
				HSTSMaxAge:            hstsMaxAge(cfg.SessionCookieSecure),
				HSTSExcludeSubdomains: false,
				ContentSecurityPolicy: "default-src 'none'; frame-ancestors 'none'; base-uri 'none'",
				ReferrerPolicy:        "no-referrer",
			}))

			frontendOrigin := cfg.FrontendOrigin()
			// The Twitch extension panel is a browser page on a Twitch-owned origin
			// (https://<client id>.ext-twitch.tv) that posts signed Bits receipts to
			// /api/overlay/extension. Allow that origin through CORS when the
			// extension is configured; the endpoint authenticates by JWT, not cookie.
			allowedOrigins := []string{frontendOrigin}
			if extOrigin := cfg.ExtensionAllowedOrigin(); extOrigin != "" {
				allowedOrigins = append(allowedOrigins, extOrigin)
			}
			e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
				AllowOrigins:     allowedOrigins,
				AllowMethods:     []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
				AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization, jwt.CSRFHeaderName, echo.HeaderXRequestID},
				ExposeHeaders:    []string{echo.HeaderXRequestID},
				AllowCredentials: true,
				MaxAge:           3600,
			}))

			api := e.Group("/api")
			api.GET("/healthz", func(c echo.Context) error {
				return c.String(http.StatusOK, "ok")
			})
			routes.RegisterRoutes(svc, api)

			// Backfill/repair overlay cheer+redemption subscriptions for every
			// broadcaster who currently runs an overlay (holds a device token), so
			// a device that outlived its subscriptions -- or the relay having just
			// been configured -- is reconciled at boot. No-op when the relay is
			// unconfigured; runs detached so it never blocks startup.
			go overlay.ReconcileOverlaySubscriptions(ctx, cfg, overlayrelay.NewService(svc), svc.Logger())

			svc.RegisterShutdownHandler(func(shutdownContext context.Context) error {
				svc.Logger().Info("graceful shutdown initiated")
				if err := e.Shutdown(shutdownContext); err != nil {
					svc.Logger().Errorf("REST shutdown failed: %s", err.Error())
				}

				p := pool.New().WithContext(shutdownContext)
				for _, closeConnection := range []func() error{svc.Postgres().Close} {
					closeConnection := closeConnection
					p.Go(func(_ context.Context) error { return closeConnection() })
				}
				if err := p.Wait(); err != nil {
					svc.Logger().Errorf("connection shutdown failed: %s", err.Error())
				}
				return nil
			})

			address := fmt.Sprintf("%s:%s", cfg.ListenAddress, cfg.APIServerPort)
			svc.Logger().Infof("starting REST server on %s", address)
			if err := e.Start(address); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		},
	)
}

func hstsMaxAge(secureCookies bool) int {
	if !secureCookies {
		return 0
	}
	return 31536000
}
