package nivek

import (
	"fmt"
	"context"
	"strings"
	"strconv"

	"github.com/upper/db/v4/adapter/postgresql"
	"github.com/sirupsen/logrus"
	"github.com/suuuth/nivek/internal/libraries/config"
	"github.com/suuuth/nivek/internal/libraries/conman"
	"github.com/suuuth/nivek/internal/libraries/abstractservice"
)

type nivekServiceImpl struct {
	abstractservice.Service

	serviceConfig  NivekServiceConfig
	commonConfig   config.Config
	customConfig   interface{}

	postgresManager conman.PostgresConnectionManager
	logger          *logrus.Logger
}

func NewNivekService(serviceConfig NivekServiceConfig) NivekService {
	return &nivekServiceImpl{
		Service: abstractservice.NewService(),
		
		postgresManager: conman.NewPostgresConnectionManager(logrus.New()),
		serviceConfig:   serviceConfig,
		logger:          logrus.New(),
	}
}

func (s *nivekServiceImpl) Postgres() conman.PostgresConnectionManager {
	if s.postgresManager == nil {
		s.logger.Fatal("postgres manager is not initialized")
	}
	return s.postgresManager
}

func (s *nivekServiceImpl) Logger() *logrus.Logger {
	return s.logger
}

func (s *nivekServiceImpl) CommonConfig() config.Config {
	return s.commonConfig
}

func (s *nivekServiceImpl) CustomConfig() interface{} {
	return s.customConfig
}

func (s *nivekServiceImpl) ReplaceCustomConfig(config interface{}) {
	s.customConfig = config
}

// Run executes main service logic
func (s *nivekServiceImpl) Run(logic ...func(context.Context) error) error {
	ctx := context.Background()

	if s.serviceConfig.UsePSQL {
		if s.serviceConfig.StartupConnectionsPostgres == nil {
			s.serviceConfig.StartupConnectionsPostgres = make(map[string]*conman.PostgresConnectionOptions)
		}

		if !s.serviceConfig.RequireStartupConnections && s.commonConfig.Postgres.Database != "" && 
		s.serviceConfig.StartupConnectionsPostgres[conman.DefaultConnection] == nil {

			host := s.commonConfig.Postgres.Host
			if !strings.Contains(host, ":") {
				host += ":" + strconv.Itoa(s.commonConfig.Postgres.Port)
			}

			options := map[string]string{
				"sslmode":     s.commonConfig.Postgres.SSLMode,
				"sslcert":     s.commonConfig.Postgres.SSLCert,
				"sslkey":      s.commonConfig.Postgres.SSLKey,
				"sslrootcert": s.commonConfig.Postgres.SSLRootCert,
			}

			if s.commonConfig.AppName != "" {
				options["application_name"] = s.commonConfig.AppName
			}

			var maxIdleConnections int
			if s.commonConfig.Postgres.MaxIdleConnections > 0 {
				maxIdleConnections = s.commonConfig.Postgres.MaxIdleConnections
			}

			s.serviceConfig.StartupConnectionsPostgres[conman.DefaultConnection] = &conman.PostgresConnectionOptions{
				ConnectionURL: postgresql.ConnectionURL{
					User:     s.commonConfig.Postgres.Username,
					Password: s.commonConfig.Postgres.Password,
					Database: s.commonConfig.Postgres.Database,
					Host:     host,
					Options:  options,
				},
				MaxConnections:        s.commonConfig.Postgres.MaxConnections,
				MaxIdleConnections:    maxIdleConnections,
				MaxTransactionRetries: s.commonConfig.Postgres.MaxTransactionRetries,
			}
		}

		s.postgresManager = conman.NewPostgresConnectionManager(s.logger)

		for name, options := range s.serviceConfig.StartupConnectionsPostgres {
			if options == nil {
				continue
			}

			if _, err := s.postgresManager.NewConnection(name, *options); err != nil {
				return err
			}
		}

		if !s.serviceConfig.SkipRegisteringShutdownHandlers {
			s.RegisterShutdownHandler(func(_ context.Context) error {
				if err := s.postgresManager.Close(); err != nil {
					return fmt.Errorf("failed to disconnect from postgres: %s", err)
				}
				return nil
			})
		}
	}

	return s.RunContext(ctx, logic...)
}
