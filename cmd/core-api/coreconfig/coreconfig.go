// Package coreconfig holds the core-api process config struct. It lives in its
// own package so endpoint packages can import the struct type for type
// assertion off nivek.NivekService.CustomConfig() without creating an import
// cycle through cmd/core-api's main package.
package coreconfig

import (
	"log"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

var staticCoreApiConfig *CoreApiConfig

func GetCoreApiConfig() CoreApiConfig {
	if staticCoreApiConfig == nil {
		parsed := Parse()
		staticCoreApiConfig = &parsed
	}

	return *staticCoreApiConfig
}

func Parse() (config CoreApiConfig) {
	if err := godotenv.Load(); err != nil {
		log.Print("No .env file found, using environment variables")
	}
	envconfig.MustProcess("", &config)
	return config
}

type CoreApiConfig struct {
	ApiServerPort string `envconfig:"CORE_API_PORT" default:""`
	ListenAddress string `envconfig:"CORE_API_LISTEN_ADDRESS" default:""`

	TwitchEventSubSecret string `envconfig:"TWITCH_EVENTSUB_SECRET" default:""`
	TwitchClientID       string `envconfig:"TWITCH_CLIENT_ID" default:""`
	TwitchClientSecret   string `envconfig:"TWITCH_CLIENT_SECRET" default:""`
	TwitchRedirectURI    string `envconfig:"TWITCH_REDIRECT_URI" default:""`
	FrontendBaseURL      string `envconfig:"FRONTEND_BASE_URL" default:""`

	// Bot listener, reached over the docker gateway. Default is the confirmed
	// bridge gateway IP for the twitch-bot host daemon's :8090 listener.
	BotInternalURL string `envconfig:"BOT_INTERNAL_URL" default:"http://172.19.0.1:8090"`
	BotAPIHMACKey  string `envconfig:"BOT_API_HMAC_KEY" default:""`
}
