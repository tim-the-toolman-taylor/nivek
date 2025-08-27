package main

import (
	"github.com/sirupsen/logrus"

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
	err := godotenv.Load(".env")
	if err != nil {
		logrus.Warningf("Error loading .env file: %s", err.Error())
	}
	envconfig.MustProcess("", &config)
	return config
}

type CoreApiConfig struct {
	ApiServerPort string `envconfig:"CORE_API_PORT" default:""`
	ListenAddress string `envconfig:"CORE_API_LISTEN_ADDRESS" default:""`
}
