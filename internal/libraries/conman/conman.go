package conman

const DefaultConnection = "default"

type ConnectionManager[V any, O any] interface {
	// NewConnection creates a new connection
	NewConnection(name string, options O) (conn V, err error)

	// GetConnection returns an existing connection, panics when there is no such connection
	GetConnection(name string) (conn V)

	// GetDefaultConnection returns the default connection, panics when there is no default connection
	GetDefaultConnection() V

	// CloseConnection closes an existing connection
	CloseConnection(name string) (err error)

	// Close closes all connections
	Close() (err error)
}
