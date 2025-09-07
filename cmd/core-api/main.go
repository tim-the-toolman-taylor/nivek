package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/sourcegraph/conc/pool"
	"github.com/suuuth/nivek/internal/libraries/nivek"
)

func main() {
	nivek.Bootstrap(
		nivek.BootstrapParameters{
			NivekServiceConfig: nivek.NivekServiceConfig{
				UsePSQL: true,

				//
				// Startup connections

				RequireStartupConnections:  true,
				StartupConnectionsPostgres: nivek.GetStartupConnectionsForPostgres(),
			},
			CustomConfig: GetCoreApiConfig(),
		},
		func(nivek nivek.NivekService, ctx context.Context) error {
			// Type assertion to convert interface{} to CoreApiConfig
			cfg, ok := nivek.CustomConfig().(CoreApiConfig)
			if !ok {
				panic("failed to assert custom config")
			}

			fmt.Println("")
			fmt.Println("====================================================")
			fmt.Println("====================================================")
			fmt.Println("Hello World! Signed - ", nivek.CommonConfig().AppName)
			fmt.Println("====================================================")
			fmt.Println("====================================================")
			fmt.Println("")

			//
			// Start the API server
			e := echo.New()

			//
			// Middleware
			// e.Use(nivekmiddleware.NivekMiddleware(nivek).Middleware())

			e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
				AllowOrigins: []string{"http://localhost"},
				AllowMethods: []string{echo.GET, echo.POST, echo.PUT, echo.DELETE},
			}))

			//
			// Register REST routes
			RegisterRoutes(nivek, e)

			//
			// Graceful shutdown
			nivek.RegisterShutdownHandler(func(ctx context.Context) error {
				nivek.Logger().Infof("graceful shutdown - initiated")

				// wait for requests to complete
				if err := e.Shutdown(context.Background()); err != nil {
					nivek.Logger().Errorf("graceful shutdown - error occurred during REST shutdown: %s", err.Error())
				}

				nivek.Logger().Infof("graceful shutdown - closing connections")

				closers := []func() error{
					nivek.Postgres().Close,
				}

				p := pool.New().WithContext(context.Background())

				for i := range closers {
					closer := closers[i]

					p.Go(func(_ context.Context) error {
						return closer()
					})
				}

				// flush remaining data and close connections
				if err := p.Wait(); err != nil {
					nivek.Logger().Errorf("failed to close connections: %s", err.Error())
				}

				nivek.Logger().Infof("graceful shutdown - done")

				return nil
			})

			nivek.Logger().Infof("starting REST server on port %s", cfg.ApiServerPort)

			if err := e.Start(fmt.Sprintf("%s:%s", cfg.ListenAddress, cfg.ApiServerPort)); err != nil {
				if !errors.Is(err, http.ErrServerClosed) {
					return err
				}
			}

			return nil
		},
	)
}
