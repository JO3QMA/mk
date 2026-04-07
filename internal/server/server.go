package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/hibiken/asynq"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/shiroha-a/mk/internal/config"
	"github.com/shiroha-a/mk/internal/core/cache"
	"github.com/shiroha-a/mk/internal/queue"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/shiroha-a/mk/internal/server/middleware"
	"gorm.io/gorm"
)

// Server represents the HTTP server.
type Server struct {
	echo        *echo.Echo
	config      *config.Config
	db          *gorm.DB
	redis       *cache.RedisClients
	auth        *middleware.AuthMiddleware
	queueClient *queue.Client
	queueServer *queue.Server
}

// New creates a new Server.
func New(cfg *config.Config, db *gorm.DB, redis *cache.RedisClients) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Global middleware
	e.Use(echomw.Recover())
	e.Use(echomw.RequestID())
	e.Use(echomw.LoggerWithConfig(echomw.LoggerConfig{
		Format: "${time_rfc3339} ${method} ${uri} ${status} ${latency_human}\n",
	}))
	e.Use(echomw.CORSWithConfig(echomw.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
	}))

	userRepo := repository.NewUserRepository(db)
	accessTokenRepo := repository.NewAccessTokenRepository(db)

	auth := middleware.NewAuthMiddleware(userRepo, accessTokenRepo)
	e.Use(auth.Authenticate())

	// asynq セットアップ: redisForJobQueue にぶら下げる。
	redisOpt := asynq.RedisClientOpt{
		Addr:     fmt.Sprintf("%s:%d", cfg.RedisForJobQueue.Host, cfg.RedisForJobQueue.Port),
		Password: cfg.RedisForJobQueue.Pass,
		DB:       cfg.RedisForJobQueue.DB,
		Username: cfg.RedisForJobQueue.Username,
	}
	concurrency := 16
	if cfg.DeliverJobConcurrency != nil && *cfg.DeliverJobConcurrency > 0 {
		concurrency = *cfg.DeliverJobConcurrency
	}
	queueClient := queue.NewClient(redisOpt)
	queueServer := queue.NewServer(redisOpt, queue.ServerConfig{Concurrency: concurrency})

	s := &Server{
		echo:        e,
		config:      cfg,
		db:          db,
		redis:       redis,
		auth:        auth,
		queueClient: queueClient,
		queueServer: queueServer,
	}

	s.setupRoutes()

	return s
}

// Start begins listening on the configured port and launches the asynq worker.
func (s *Server) Start() error {
	if err := s.queueServer.Start(); err != nil {
		return fmt.Errorf("start queue worker: %w", err)
	}
	addr := fmt.Sprintf(":%d", s.config.Port)
	slog.Info("starting Misskey server", "addr", addr, "url", s.config.URL)
	return s.echo.Start(addr)
}

// Shutdown gracefully shuts down the server and the asynq worker.
func (s *Server) Shutdown(ctx context.Context) error {
	s.queueServer.Shutdown()
	if err := s.queueClient.Close(); err != nil {
		slog.Warn("queue client close failed", "err", err)
	}
	return s.echo.Shutdown(ctx)
}
