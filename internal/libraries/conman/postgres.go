package conman

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/upper/db/v4"
	"github.com/upper/db/v4/adapter/postgresql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type PostgresConnectionOptions struct {
	// CloudSqlOptions the underlying connection url
	postgresql.ConnectionURL

	// MaxConnections the maximum number of connections
	MaxConnections int

	// MaxIdleConnections the minimum number of idle connections
	MaxIdleConnections int

	// MaxTransactionRetries the maximum number of times a transaction can be retried
	MaxTransactionRetries int
}

type connectionAndPostgresConnectionOptions struct {
	options    PostgresConnectionOptions
	connection db.Session
}

//go:generate mockgen -destination=./mocks/DBSession.go -package=mock_conman github.com/upper/db/v4 Session
//go:generate mockgen -destination=./mocks/DBCollection.go -package=mock_conman github.com/upper/db/v4 Collection
//go:generate mockgen -destination=./mocks/DBResult.go -package=mock_conman github.com/upper/db/v4 Result

// PostgresConnectionManager
//
//go:generate mockgen -source=./postgres_conman.go -destination=./mocks/PostgresConnectionManager.go PostgresConnectionManager
type PostgresConnectionManager interface {
	ConnectionManager[db.Session, PostgresConnectionOptions]
	gormConnector
}

type postgresConnectionManagerImpl struct {
	logger      *logrus.Logger
	mutex       sync.RWMutex
	connections map[string]connectionAndPostgresConnectionOptions
}

func NewPostgresConnectionManager(logger *logrus.Logger) PostgresConnectionManager {
	return &postgresConnectionManagerImpl{
		logger:      logger,
		connections: make(map[string]connectionAndPostgresConnectionOptions),
	}
}

// NewConnection creates a new connection
func (m *postgresConnectionManagerImpl) NewConnection(
	name string,
	options PostgresConnectionOptions,
) (db.Session, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	session, err := m.newDirectConnection(name, options)
	if err != nil {
		return nil, err
	}

	if session != nil {
		if options.MaxConnections > 0 {
			m.logger.Infof("max postgres connections for %s: %d", name, options.MaxConnections)
			session.SetMaxOpenConns(options.MaxConnections)
		}

		if options.MaxIdleConnections > 0 {
			m.logger.Infof("max idle postgres connections for %s: %d", name, options.MaxIdleConnections)
			session.SetMaxIdleConns(options.MaxIdleConnections)
		}

		if options.MaxTransactionRetries > 0 {
			m.logger.Infof("max postgres transaction retries for %s: %d", name, options.MaxTransactionRetries)
			session.SetMaxTransactionRetries(options.MaxTransactionRetries)
		}

		m.connections[name] = connectionAndPostgresConnectionOptions{
			options:    options,
			connection: session,
		}
	}

	return session, err
}

// GetConnection returns an existing connection, panics when there is no such connection
func (m *postgresConnectionManagerImpl) GetConnection(name string) (session db.Session) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if sessionData, exists := m.connections[name]; !exists {
		m.logger.Fatalf("postgres connection %s was not initialized", name)
	} else {
		session = sessionData.connection
	}
	return
}

// GetDefaultConnection retrieves the default connection
func (m *postgresConnectionManagerImpl) GetDefaultConnection() db.Session {
	return m.GetConnection(DefaultConnection)
}

// CloseConnection closes an existing connection
func (m *postgresConnectionManagerImpl) CloseConnection(name string) (err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if sessionData, exists := m.connections[name]; exists {
		if err = sessionData.connection.Close(); err == nil {
			delete(m.connections, name)
		}
	}
	return
}

// Close closes all connections
func (m *postgresConnectionManagerImpl) Close() (err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for _, sessionData := range m.connections {
		if err = sessionData.connection.Close(); err != nil {
			return
		}
	}
	m.connections = make(map[string]connectionAndPostgresConnectionOptions)
	return
}

func (m *postgresConnectionManagerImpl) Gorm(
	name string,
	createBatchSize int,
	slowThreshold time.Duration,
) (*gorm.DB, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if sessionData, exists := m.connections[name]; !exists {
		return nil, fmt.Errorf("postgres connection %s was not initialized", name)
	} else {
		if gormDb, err := getGormDb(
			postgres.New(postgres.Config{
				Conn: sessionData.connection.Driver().(*sql.DB),
			}),
			m.logger,
			createBatchSize,
			slowThreshold,
		); err != nil {
			return nil, err
		} else if err := m.afterGormInit(gormDb, sessionData.options); err != nil {
			return nil, err
		} else {
			return gormDb, nil
		}
	}
}

func (m *postgresConnectionManagerImpl) CustomGorm(name string, config gorm.Config) (*gorm.DB, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if sessionData, exists := m.connections[name]; !exists {
		return nil, fmt.Errorf("postgres connection %s was not initialized", name)
	} else {
		if gormDb, err := getCustomGormDb(
			postgres.New(postgres.Config{
				Conn: m.GetConnection(name).Driver().(*sql.DB),
			}),
			m.logger,
			config,
		); err != nil {
			return nil, err
		} else if err := m.afterGormInit(gormDb, sessionData.options); err != nil {
			return nil, err
		} else {
			return gormDb, nil
		}
	}
}

func (m *postgresConnectionManagerImpl) afterGormInit(gormDb *gorm.DB, options PostgresConnectionOptions) error {
	if connPool, err := gormDb.DB(); err != nil {
		return err
	} else {
		// Setup connection pool options
		connPool.SetMaxIdleConns(options.MaxIdleConnections)
		connPool.SetMaxOpenConns(options.MaxConnections)
		connPool.SetConnMaxIdleTime(5 * time.Minute)
		connPool.SetConnMaxLifetime(time.Hour)
	}

	return nil
}

func (m *postgresConnectionManagerImpl) newDirectConnection(
	name string,
	options PostgresConnectionOptions,
) (session db.Session, err error) {
	for i := 0; i < 3; i++ {
		m.logger.Infof("connecting to postgres %s: %s", name, options.Host)
		if session, err = postgresql.Open(options); err != nil {
			if i < 2 {
				m.logger.Errorf("failed to open postgres connection for %s: %s", name, err.Error())
				m.logger.Infof("retrying postgres connection after delay...")
				time.Sleep(1_500 * time.Millisecond)
			} else {
				err = fmt.Errorf("failed to open postgres connection for %s: %w", name, err)
				return
			}
		} else {
			break
		}
	}

	return
}
