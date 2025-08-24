package nivek

import (
	"context"
	"fmt"
	"time"
	"sync/atomic"
	"runtime"
	
	"github.com/upper/db/v4/adapter/postgresql"
	"github.com/suuuth/nivek/internal/libraries/conman"
	"github.com/suuuth/nivek/internal/libraries/config"
)

type BootstrapParameters struct {
	NivekServiceConfig
	CustomConfig any
}

func Bootstrap(
	parameters BootstrapParameters,
	serviceLogic func(NivekService, context.Context) error,
) {
	nivek := NewNivekService(parameters.NivekServiceConfig)

	if err := nivek.Run(func(ctx context.Context) error {
		setEngine(nivek)

		if serviceLogic == nil {
			return fmt.Errorf("serviceLogic was not defined")
		}

		goRoutineLeakDetection(nivek)

		nivek.ReplaceCustomConfig(parameters.CustomConfig)

		return serviceLogic(nivek, ctx)
	}); err != nil {
		nivek.Logger().Fatalf("failed to run: %s", err.Error())
	}
}

func GetStartupConnectionsForPostgres() map[string]*conman.PostgresConnectionOptions {
	return map[string]*conman.PostgresConnectionOptions{
		conman.DefaultConnection: {
			ConnectionURL: postgresql.ConnectionURL{
				User:     config.GetConfig().Postgres.Username,
				Password: config.GetConfig().Postgres.Password,
				Database: config.GetConfig().Postgres.Database,
				Host:     fmt.Sprintf("%s:%d", config.GetConfig().Postgres.Host, config.GetConfig().Postgres.Port),
				Options: map[string]string{
					"application_name": config.GetConfig().AppName,
					"sslmode":          config.GetConfig().Postgres.SSLMode,
					"sslcert":          config.GetConfig().Postgres.SSLCert,
					"sslkey":           config.GetConfig().Postgres.SSLKey,
					"sslrootcert":      config.GetConfig().Postgres.SSLRootCert,
				},
			},
			MaxConnections:        config.GetConfig().Postgres.MaxConnections,
			MaxIdleConnections:    config.GetConfig().Postgres.MaxIdleConnections,
			MaxTransactionRetries: config.GetConfig().Postgres.MaxTransactionRetries,
		},
	}
}

func goRoutineLeakDetection(s NivekService) {
	timer := time.NewTicker(60 * time.Second)
	s.RegisterShutdownHandler(func(ctx context.Context) error {
		timer.Stop()
		return nil
	})

	go func() {
		lastRoutineLeakNumber := &atomic.Int64{}

		for range timer.C {
			routines := runtime.NumGoroutine()

			// generate a warning if there are an usually high number of go routines
			if routines >= 5_000 {
				s.Logger().Warnf(
					"detected an unusually high number of go routines, "+
						"monitor these logs to determine if there is a leak - "+
						"current: %d, previous: %d",
					routines,
					lastRoutineLeakNumber.Load(),
				)
			}

			lastRoutineLeakNumber.Store(int64(routines))
		}
	}()
}
