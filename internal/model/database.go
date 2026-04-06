package model

import (
	"fmt"
	"log/slog"

	"github.com/misskey-dev/misskey-go/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewDatabase creates a new GORM database connection.
func NewDatabase(cfg *config.Config) (*gorm.DB, error) {
	logLevel := logger.Warn
	if cfg.Logging != nil && cfg.Logging.SQL != nil && cfg.Logging.SQL.EnableQueryParamLog {
		logLevel = logger.Info
	}

	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
		// TypeORM互換: テーブル名をそのまま使う
		DisableNestedTransaction: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	slog.Info("connected to PostgreSQL",
		"host", cfg.DB.Host,
		"port", cfg.DB.Port,
		"db", cfg.DB.DB,
	)

	return db, nil
}
