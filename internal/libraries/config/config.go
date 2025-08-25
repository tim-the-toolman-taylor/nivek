package config

import (
	"github.com/kelseyhightower/envconfig"
)

var staticConfig *Config

func GetConfig() Config {
	if staticConfig == nil {
		parsed := Parse()
		staticConfig = &parsed
	}

	return *staticConfig
}

func Parse() (config Config) {
	envconfig.MustProcess("", &config)
	return config
}

type Config struct {
	AppName string `envconfig:"APP_NAME" default:"test"`
	Postgres PostgresConfig
}

type PostgresConfig struct {
	Host     string `envconfig:"POSTGRES_HOST" default:"127.0.0.1"`
	Port     int    `envconfig:"POSTGRES_PORT" default:"5432"`
	Username string `envconfig:"POSTGRES_USERNAME" default:""`
	Password string `envconfig:"POSTGRES_PASSWORD" default:""`
	Database string `envconfig:"POSTGRES_DATABASE" default:""`

	SSLMode     string `envconfig:"POSTGRES_SSL_MODE" default:"disable"`
	SSLCert     string `envconfig:"POSTGRES_SSL_CERT" default:""`
	SSLKey      string `envconfig:"POSTGRES_SSL_KEY" default:""`
	SSLRootCert string `envconfig:"POSTGRES_SSL_ROOT_CERT" default:""`

	MaxConnections        int `envconfig:"POSTGRES_MAX_CONNECTIONS" default:"2"`
	MaxIdleConnections    int `envconfig:"POSTGRES_MAX_IDLE_CONNECTIONS" default:"1"`
	MaxTransactionRetries int `envconfig:"POSTGRES_MAX_TRANSACTION_RETRIES" default:"0"`
}
