package conman

import (
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type gormConnector interface {
	// Gorm returns a gorm DB for the specified connection time
	Gorm(
		name string,
		createBatchSize int,
		slowThreshold time.Duration,
	) (*gorm.DB, error)
}

// getGormDb generic gorm connector
func getGormDb(
	dialector gorm.Dialector,
	logrusLogger *logrus.Logger,
	createBatchSize int,
	slowThreshold time.Duration,
) (*gorm.DB, error) {
	// setup default configurations
	config := configureGorm(logrusLogger, logger.Config{})

	gormDb, err := gorm.Open(dialector, config)

	if err != nil {
		return nil, fmt.Errorf("failed to initialize gorm: %w", err)
	}

	return gormDb, nil
}

// getCustomGormDb custom gorm connector
func getCustomGormDb(
	dialector gorm.Dialector,
	logrusLogger *logrus.Logger,
	config gorm.Config,
) (*gorm.DB, error) {
	gormDb, err := gorm.Open(dialector, &config)

	if err != nil {
		return nil, fmt.Errorf("failed to initialize gorm: %w", err)
	}

	return gormDb, nil
}

func configureGorm(
	logrusLogger *logrus.Logger,
	loggerConfig logger.Config,
) *gorm.Config {
	if loggerConfig.SlowThreshold <= 0 {
		loggerConfig.SlowThreshold = 200 * time.Millisecond
	}

	if loggerConfig.LogLevel <= 0 {
		loggerConfig.LogLevel = logger.Warn
	}

	return &gorm.Config{
		CreateBatchSize: 1_000,
		Logger: logger.New(
			NewLogrusGormLogger(logrusLogger, logrus.WarnLevel, 0),
			loggerConfig,
		),
	}
}

func NewLogrusGormLogger(logger *logrus.Logger, level logrus.Level, maxCharacters int) logger.Writer {
	if maxCharacters == 0 {
		maxCharacters = 1_000
	}

	return &logrusGormLogger{
		logger:        logger,
		level:         level,
		maxCharacters: maxCharacters,
	}
}

type logrusGormLogger struct {
	logger        *logrus.Logger
	level         logrus.Level
	maxCharacters int
}

func (l *logrusGormLogger) Printf(format string, args ...interface{}) {
	go func() {
		message := fmt.Sprintf(format, args...)
		if l.maxCharacters > 0 && len(message) > l.maxCharacters {
			message = message[0:l.maxCharacters] + "..."
		}

		l.logger.Log(
			l.level,
			strings.ReplaceAll(strings.ReplaceAll(message, "\t", "\\t"), "\n", "\\n"),
		)
	}()
}
