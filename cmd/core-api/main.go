package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/sourcegraph/conc/pool"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/coreconfig"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/routes"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/jwt"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
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
			e := echo.New()
			e.HideBanner = true
			e.HidePort = true
			e.Server.ReadHeaderTimeout = 5 * time.Second
			e.Server.ReadTimeout = 20 * time.Second
			e.Server.WriteTimeout = 30 * time.Second
			e.Server.IdleTimeout = 60 * time.Second

			e.Use(middleware.Recover())
			e.Use(middleware.RequestID())
			e.Use(middleware.BodyLimit("1M"))
			e.Use(middleware.Gzip())
			e.Use(middleware.SecureWithConfig(middleware.SecureConfig{
				XSSProtection:         "0",
				ContentTypeNosniff:    "nosniff",
				XFrameOptions:         "DENY",
				HSTSMaxAge:            hstsMaxAge(cfg.SessionCookieSecure),
				HSTSExcludeSubdomains: false,
				ContentSecurityPolicy: "default-src 'none'; frame-ancestors 'none'; base-uri 'none'",
				ReferrerPolicy:        "no-referrer",
			}))

			frontendOrigin := cfg.FrontendOrigin()
			e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
				AllowOrigins:     []string{frontendOrigin},
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
