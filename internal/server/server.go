package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/shiroha-a/mk/internal/config"
	"github.com/shiroha-a/mk/internal/core/cache"
	"github.com/shiroha-a/mk/internal/core/chart"
	"github.com/shiroha-a/mk/internal/queue"
	"github.com/shiroha-a/mk/internal/queue/driver"
	"github.com/shiroha-a/mk/internal/queue/driver/asynqdriver"
	"github.com/shiroha-a/mk/internal/repository"
	mksentry "github.com/shiroha-a/mk/internal/sentry"
	"github.com/shiroha-a/mk/internal/server/middleware"
	"gorm.io/gorm"
)

// Server represents the HTTP server.
type Server struct {
	echo           *echo.Echo
	config         *config.Config
	db             *gorm.DB
	redis          *cache.RedisClients
	auth           *middleware.AuthMiddleware
	queueDriver    driver.Driver
	queueClient    *queue.Client
	queueServer    *queue.Server
	queueScheduler *queue.Scheduler
	queueInspector *queue.Inspector
	chartMgmt      *chart.ManagementService

	// shutdownHooks はShutdown()時にqueue/HTTP echoより先に呼ばれる
	// ティッカー系ジョブの停止用。publisher goroutineをcleanに止める。
	shutdownHooks []func()
}

// registerShutdownHook registers fn to be invoked during Shutdown.
// Hooks run in registration order before the asynq / echo shutdown.
func (s *Server) registerShutdownHook(fn func()) {
	s.shutdownHooks = append(s.shutdownHooks, fn)
}

// New creates a new Server.
func New(cfg *config.Config, db *gorm.DB, redis *cache.RedisClients) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// trustProxyからIPExtractorを構成
	if nets := config.ParseTrustProxy(cfg.TrustProxy); len(nets) > 0 {
		var opts []echo.TrustOption
		for _, n := range nets {
			opts = append(opts, echo.TrustIPRange(n))
		}
		e.IPExtractor = echo.ExtractIPFromXFFHeader(opts...)
	}

	// Global middleware
	e.Use(echomw.Recover())
	// Sentry middleware は Recover の直後に置く: panic を hub に送ったあと
	// Recover に巻き戻し、5xx の最終整形は echo に任せる。
	e.Use(mksentry.Middleware(cfg))
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

	// queue driver セットアップ: redisForJobQueue にぶら下げる。
	// Host が UNIX domain socket パス ("/" 始まり) なら Network を "unix" に
	// 切り替える (asynqdriver.BuildRedisOpt が判定する)。
	concurrency := 16
	if cfg.DeliverJobConcurrency != nil && *cfg.DeliverJobConcurrency > 0 {
		concurrency = *cfg.DeliverJobConcurrency
	}
	queueDriver := asynqdriver.New(
		asynqdriver.BuildRedisOpt(cfg.RedisForJobQueue),
		asynqdriver.ServerConfig{Concurrency: concurrency},
	)
	queueClient := queue.NewClient(queueDriver)
	queueServer := queue.NewServer(queueDriver)
	queueScheduler := queue.NewScheduler(queueDriver)
	queueInspector := queue.NewInspector(queueDriver)

	s := &Server{
		echo:           e,
		config:         cfg,
		db:             db,
		redis:          redis,
		auth:           auth,
		queueDriver:    queueDriver,
		queueClient:    queueClient,
		queueServer:    queueServer,
		queueScheduler: queueScheduler,
		queueInspector: queueInspector,
	}

	s.setupRoutes()

	return s
}

// Handler returns the underlying http.Handler for use with httptest.
// E2Eテスト等でサーバーを外部から起動する場合に使う���
func (s *Server) Handler() http.Handler {
	return s.echo
}

// Start begins listening on the configured port (or UNIX domain socket) and
// launches the asynq worker.
//
// If s.config.Socket is non-empty the HTTP server binds to that path instead
// of a TCP port. This matches Misskey 本家 YAML の `socket` / `chmodSocket`
// 設定と同じ運用感覚で使える。
func (s *Server) Start() error {
	if err := s.queueServer.Start(); err != nil {
		return fmt.Errorf("start queue worker: %w", err)
	}
	if s.queueScheduler != nil {
		if err := s.queueScheduler.RegisterChartJobs(); err != nil {
			slog.Warn("chart scheduler register failed", "err", err)
		}
		if err := s.queueScheduler.RegisterInstanceRefreshJob(); err != nil {
			slog.Warn("instance refresh scheduler register failed", "err", err)
		}
		if err := s.queueScheduler.RegisterRetentionJob(); err != nil {
			slog.Warn("retention scheduler register failed", "err", err)
		}
		if err := s.queueScheduler.Start(); err != nil {
			slog.Warn("scheduler start failed", "err", err)
		}
	}
	if s.chartMgmt != nil {
		if err := s.chartMgmt.Start(context.Background()); err != nil {
			slog.Warn("chart management service start failed", "err", err)
		}
	}

	if s.config.Socket != "" {
		ln, err := config.ListenUnixSocket(s.config.Socket, s.config.ChmodSocket)
		if err != nil {
			return err
		}
		s.echo.Listener = ln
		slog.Info("starting Misskey server",
			"socket", s.config.Socket, "url", s.config.URL)
		// Echo.Start は内部で net.Listen してしまうので、ここでは Start では
		// なく Serve を使って既に張った listener を使う。
		if err := s.echo.Server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}

	addr := fmt.Sprintf(":%d", s.config.Port)
	slog.Info("starting Misskey server", "addr", addr, "url", s.config.URL)
	return s.echo.Start(addr)
}

// Shutdown gracefully shuts down the server, the asynq worker and
// any background services such as the chart management loop.
func (s *Server) Shutdown(ctx context.Context) error {
	// 登録順にshutdown hookを走らせる。publisher goroutineをclean停止。
	for _, hook := range s.shutdownHooks {
		hook()
	}
	if s.chartMgmt != nil {
		s.chartMgmt.Stop(ctx)
	}
	if s.queueScheduler != nil {
		s.queueScheduler.Shutdown()
	}
	s.queueServer.Shutdown()
	if err := s.queueClient.Close(); err != nil {
		slog.Warn("queue client close failed", "err", err)
	}
	err := s.echo.Shutdown(ctx)
	// UDS listen していた場合、Shutdown で net.Listener.Close() は呼ばれる
	// が、ソケットファイル自体は残るので明示的に unlink しておく。
	if rmErr := config.RemoveUnixSocket(s.config.Socket); rmErr != nil {
		slog.Warn("failed to remove socket file", "socket", s.config.Socket, "err", rmErr)
	}
	return err
}

// setChartManagement registers the chart management service so its
// save loop is started/stopped together with the HTTP server. Called
// from setupRoutes after the chart engines are constructed.
func (s *Server) setChartManagement(m *chart.ManagementService) {
	s.chartMgmt = m
}
