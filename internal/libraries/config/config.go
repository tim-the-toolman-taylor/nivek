package config

import (
	"log"

	"github.com/joho/godotenv"
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
	if err := godotenv.Load(); err != nil {
		// stdlib log on purpose: config parsing runs before the nivek
		// service (and its logger) exists.
		log.Print("No .env file found, using environment variables")
	}
	envconfig.MustProcess("", &config)
	return config
}

type Config struct {
	AppName string `envconfig:"APP_NAME" default:"default"`
	Windy   WindyConfig
	IPInfo  IPInfoConfig

	Postgres PostgresConfig
}

type PostgresConfig struct {
	Host     string `envconfig:"POSTGRES_HOST" default:"0.0.0.0"`
	Port     int    `envconfig:"POSTGRES_PORT" default:"5432"`
	Username string `envconfig:"POSTGRES_USERNAME" default:""`
	Password string `envconfig:"POSTGRES_PASSWORD" default:""`
	Database string `envconfig:"POSTGRES_DB" default:"nivek"`

	SSLMode     string `envconfig:"POSTGRES_SSL_MODE" default:"disable"`
	SSLCert     string `envconfig:"POSTGRES_SSL_CERT" default:""`
	SSLKey      string `envconfig:"POSTGRES_SSL_KEY" default:""`
	SSLRootCert string `envconfig:"POSTGRES_SSL_ROOT_CERT" default:""`

	MaxConnections        int `envconfig:"POSTGRES_MAX_CONNECTIONS" default:"2"`
	MaxIdleConnections    int `envconfig:"POSTGRES_MAX_IDLE_CONNECTIONS" default:"1"`
	MaxTransactionRetries int `envconfig:"POSTGRES_MAX_TRANSACTION_RETRIES" default:"0"`
}

type WindyConfig struct {
	Token string `envconfig:"WINDY_API_TOKEN" default:""`
}

type IPInfoConfig struct {
	Token string `envconfig:"IP_INFO_API_TOKEN" default:""`
}
