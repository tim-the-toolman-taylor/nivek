package nivek

import "github.com/suuuth/nivek/internal/libraries/conman"

type NivekServiceConfig struct {
	UsePSQL bool

	// RequireStartupConnections when true the "default" connections will not be added automatically, be aware that
	// some services (distributed locker, merchant provider) may require the "default" connection to be available,
	// and so you will need to add that manually as needed
	RequireStartupConnections bool

	// StartupConnectionsPostgres default Postgres connections
	StartupConnectionsPostgres map[string]*conman.PostgresConnectionOptions

	// SkipRegisteringShutdownHandlers when true the service will not register any shutdown handlers
	SkipRegisteringShutdownHandlers bool
}
