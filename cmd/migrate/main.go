package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/url"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/shiroha-a/mk/internal/config"
)

func main() {
	configPath := flag.String("config", ".config/default.yml", "path to configuration file")
	direction := flag.String("direction", "up", "migration direction: up or down")
	steps := flag.Int("steps", 0, "number of steps (0 = all)")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	var dbURL string
	if config.IsUnixSocketPath(cfg.DB.Host) {
		// UDS の場合はホスト部を空にして host クエリにソケットディレクトリを渡す。
		// postgres://...@/var/run/postgresql:5432/... の形式は libpq が UDS として
		// 解釈できず TCP localhost にフォールバックするため。
		dbURL = fmt.Sprintf("postgres:///%s?host=%s&port=%d&user=%s&password=%s&sslmode=disable",
			cfg.DB.DB,
			url.QueryEscape(cfg.DB.Host),
			cfg.DB.Port,
			url.QueryEscape(cfg.DB.User),
			url.QueryEscape(cfg.DB.Pass),
		)
	} else {
		dbURL = fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
			cfg.DB.User, cfg.DB.Pass, cfg.DB.Host, cfg.DB.Port, cfg.DB.DB,
		)
	}

	m, err := migrate.New("file://migration", dbURL)
	if err != nil {
		slog.Error("failed to create migrator", "error", err)
		os.Exit(1)
	}
	defer m.Close()

	switch *direction {
	case "up":
		if *steps > 0 {
			err = m.Steps(*steps)
		} else {
			err = m.Up()
		}
	case "down":
		if *steps > 0 {
			err = m.Steps(-*steps)
		} else {
			err = m.Down()
		}
	default:
		slog.Error("invalid direction", "direction", *direction)
		os.Exit(1)
	}

	if err != nil && err != migrate.ErrNoChange {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}

	if err == migrate.ErrNoChange {
		slog.Info("no migration changes to apply")
	} else {
		slog.Info("migration completed", "direction", *direction)
	}
}
