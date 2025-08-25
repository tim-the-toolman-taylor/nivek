package main

import (
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
	envconfig.MustProcess("", &config)
	return config
}

type CoreApiConfig struct {
	ApiServerPort string `envconfig:"CORE_API_PORT" default:"8080"`
	ListenAddress string `envconfig:"CORE_API_LISTEN_ADDRESS" default:"0.0.0.0"`
}
