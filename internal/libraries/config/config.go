package config

var staticConfig Config

func GetConfig() Config {
	return staticConfig
}

type Config struct {
	AppName string `envconfig:"APP_NAME" default:""`
	Postgres PostgresConfig
	HTTP HTTPConfig
}

type PostgresConfig struct {
	Host     string `envconfig:"POSTGRES_HOST" default:"127.0.0.1"`
	Port     int    `envconfig:"POSTGRES_PORT" default:"5432"`
	Username string `envconfig:"POSTGRES_USERNAME" default:"nivek"`
	Password string `envconfig:"POSTGRES_PASSWORD" default:"password123!"`
	Database string `envconfig:"POSTGRES_DATABASE" default:"nivek"`

	SSLMode     string `envconfig:"POSTGRES_SSL_MODE" default:"disable"`
	SSLCert     string `envconfig:"POSTGRES_SSL_CERT" default:""`
	SSLKey      string `envconfig:"POSTGRES_SSL_KEY" default:""`
	SSLRootCert string `envconfig:"POSTGRES_SSL_ROOT_CERT" default:""`

	MaxConnections        int `envconfig:"POSTGRES_MAX_CONNECTIONS" default:"2"`
	MaxIdleConnections    int `envconfig:"POSTGRES_MAX_IDLE_CONNECTIONS" default:"1"`
	MaxTransactionRetries int `envconfig:"POSTGRES_MAX_TRANSACTION_RETRIES" default:"0"`
}

type HTTPConfig struct {
	ApiPort     string `envconfig:"HTTP_PORT_API" default:""`
}
