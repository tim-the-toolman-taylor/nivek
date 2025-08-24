package main

var staticCoreApiConfig CoreApiConfig

func GetCoreApiConfig() CoreApiConfig {
	return staticCoreApiConfig
}

type CoreApiConfig struct {
	ApiServerPort string `envconfig:"CORE_API_PORT" default:"8080"`
	ListenAddress string `envconfig:"CORE_API_LISTEN_ADDRESS" default:"0.0.0.0"`
}
