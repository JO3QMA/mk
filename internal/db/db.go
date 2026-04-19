// Package db wires the GORM PostgreSQL connection used by the rest of the
// application. Lives in its own package so its dbresolver wiring tests do not
// drag down internal/model coverage (model is mostly pure struct definitions).
package db

import (
	"fmt"
	"log/slog"
	"time"

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
		// クエリのprepared statementをコネクション単位でキャッシュし、
		// パース/プラン作成のオーバーヘッドを削減する
		PrepareStmt: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// コネクションプール設定。未指定時はWebアプリケーション向けの適正値を採用する。
	// Goのデフォルト(MaxOpenConns=0=無制限)だと高負荷時にPGバックエンド生成コストが
	// テールレイテンシに直結するため、明示的に上限を設ける。
	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	maxOpen := 25
	if cfg.DB.MaxOpenConns != nil {
		maxOpen = *cfg.DB.MaxOpenConns
	}
	maxIdle := 25
	if cfg.DB.MaxIdleConns != nil {
		maxIdle = *cfg.DB.MaxIdleConns
	}
	connMaxLifetime := 5 * time.Minute
	if cfg.DB.ConnMaxLifetime != nil {
		connMaxLifetime = time.Duration(*cfg.DB.ConnMaxLifetime) * time.Second
	}
	connMaxIdleTime := 5 * time.Minute
	if cfg.DB.ConnMaxIdleTime != nil {
		connMaxIdleTime = time.Duration(*cfg.DB.ConnMaxIdleTime) * time.Second
	}

	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(connMaxLifetime)
	sqlDB.SetConnMaxIdleTime(connMaxIdleTime)

	slog.Info("connected to PostgreSQL",
		"host", cfg.DB.Host,
		"port", cfg.DB.Port,
		"db", cfg.DB.DB,
		"maxOpenConns", maxOpen,
		"maxIdleConns", maxIdle,
		"connMaxLifetime", connMaxLifetime,
		"connMaxIdleTime", connMaxIdleTime,
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
		}).
			SetMaxOpenConns(maxOpen).
			SetMaxIdleConns(maxIdle).
			SetConnMaxLifetime(connMaxLifetime).
			SetConnMaxIdleTime(connMaxIdleTime),
		); err != nil {
			return nil, fmt.Errorf("failed to register db replicas: %w", err)
		}
		slog.Info("registered PostgreSQL read replicas", "count", len(cfg.DBSlaves))
	}

	return gdb, nil
}
