// Package db wires the GORM PostgreSQL connection used by the rest of the
// application. Lives in its own package so its dbresolver wiring tests do not
// drag down internal/model coverage (model is mostly pure struct definitions).
package db

import (
	"fmt"
	"log/slog"

	"github.com/shiroha-a/mk/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/plugin/dbresolver"
)

// New opens a GORM database connection. If cfg.DBReplications is true and
// cfg.DBSlaves is non-empty, register read replicas via the dbresolver
// plugin so SELECT queries are routed to replicas.
func New(cfg *config.Config) (*gorm.DB, error) {
	logLevel := logger.Warn
	if cfg.Logging != nil && cfg.Logging.SQL != nil && cfg.Logging.SQL.EnableQueryParamLog {
		logLevel = logger.Info
	}

	gdb, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
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

	// TS互換: dbReplications フラグを明示的に有効化しない限り dbSlaves を無視する
	if cfg.DBReplications && len(cfg.DBSlaves) > 0 {
		replicas := make([]gorm.Dialector, 0, len(cfg.DBSlaves))
		for i := range cfg.DBSlaves {
			replicas = append(replicas, postgres.Open(cfg.SlaveDSN(i)))
		}
		// dbresolver は SELECT を Replicas (RandomPolicy) へ、書き込みと
		// dbresolver.Write 明示クエリを Sources (primary) へ振り分ける。
		// Use() は内部で Initialize エラーを返しうるが現実装では起きないため
		// fail-fast でラップして上位に返す。
		if err := gdb.Use(dbresolver.Register(dbresolver.Config{
			Replicas: replicas,
			Policy:   dbresolver.RandomPolicy{},
		})); err != nil {
			return nil, fmt.Errorf("failed to register db replicas: %w", err)
		}
		slog.Info("registered PostgreSQL read replicas", "count", len(cfg.DBSlaves))
	}

	return gdb, nil
}
