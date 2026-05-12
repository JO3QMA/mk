package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shiroha-a/mk/internal/config"
	"github.com/shiroha-a/mk/internal/core/cache"
	"github.com/shiroha-a/mk/internal/model"
	mksentry "github.com/shiroha-a/mk/internal/sentry"
	"github.com/shiroha-a/mk/internal/server"
)

// healthcheckTimeout is the maximum time the -healthcheck mode waits for
// the local /healthz endpoint to respond.
const healthcheckTimeout = 3 * time.Second

func main() {
	configPath := flag.String("config", ".config/default.yml", "path to configuration file")
	healthcheckMode := flag.Bool("healthcheck", false, "perform a healthcheck (GET /healthz against the configured port) and exit 0/1")
	flag.Parse()

	if *healthcheckMode {
		os.Exit(runHealthcheck(*configPath))
	}

	// ロガーの初期化
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	slog.Info("starting Misskey (Go)", "version", config.MisskeyVersion, "mkGoVersion", config.MkGoVersion)

	// 設定ファイルの読み込み
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// pidFile が設定されていれば PID を書き込み、終了時に削除する。
	// 既存ファイルが生存中の他プロセスを指す場合は ErrAlreadyRunning で
	// 起動を拒否して二重起動を防ぐ (#497)。
	cleanupPid, err := server.WritePidFile(cfg.PidFile)
	if err != nil {
		slog.Error("failed to write pid file", "error", err)
		os.Exit(1)
	}
	defer cleanupPid()

	// Sentry init は他のサービスより前に走らせ、以降の起動エラーも捕捉対象にする。
	flushSentry, err := mksentry.Init(cfg)
	if err != nil {
		slog.Error("failed to init sentry", "error", err)
		os.Exit(1)
	}
	defer flushSentry()

	// DB接続
	db, err := model.NewDatabase(cfg)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}

	// Redis接続
	redisClients, err := cache.NewRedisClients(cfg)
	if err != nil {
		slog.Error("failed to connect to Redis", "error", err)
		os.Exit(1)
	}
	defer redisClients.Close()

	// HTTPサーバー起動
	srv, err := server.New(cfg, db, redisClients)
	if err != nil {
		slog.Error("failed to construct server", "error", err)
		os.Exit(1)
	}

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.Start(); err != nil {
			slog.Error("server error", "error", err)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10_000_000_000) // 10 seconds
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}

	fmt.Println("Misskey stopped.")
}

// runHealthcheck performs a single HTTP GET against http://127.0.0.1:<port>/healthz
// using the port resolved from the same config the running server uses.
// distroless image には wget/curl が無いので、healthcheck をこの binary
// 自身で完結させるための専用モード (#621)。
func runHealthcheck(configPath string) int {
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "healthcheck: failed to load config: %v\n", err)
		return 1
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/healthz", cfg.Port)
	client := &http.Client{Timeout: healthcheckTimeout}
	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "healthcheck: %v\n", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "healthcheck: status %d\n", resp.StatusCode)
		return 1
	}
	return 0
}
