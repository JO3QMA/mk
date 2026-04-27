package asynqdriver

import (
	"context"
	"errors"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/shiroha-a/mk/internal/queue/driver"
)

// ServerConfig configures the asynq-backed worker.
type ServerConfig struct {
	// Concurrency is the number of worker goroutines. Zero falls
	// back to 16 to match the historical default.
	Concurrency int
	// Queues maps queue name → relative priority weight. asynq
	// uses these as scheduling weights; if empty, the asynq driver
	// fills in the queue list expected by mk-go (deliver / push /
	// export / webhook / maintenance).
	Queues map[string]int
}

// Server implements driver.Server over *asynq.Server + ServeMux.
type Server struct {
	inner *asynq.Server
	mux   *asynq.ServeMux
}

// NewServer constructs the worker side of the asynq driver bound to
// the given Redis connection.
func NewServer(redisOpt asynq.RedisClientOpt, cfg ServerConfig) *Server {
	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = 16
	}
	queues := cfg.Queues
	if len(queues) == 0 {
		// Misskey-go の従来 wiring と同じ queue set を埋め直す。
		// 新しい queue を増やすときはここと queue.QueueName 等の
		// 定数追加を同時に行う。
		queues = map[string]int{
			"deliver":     1,
			"push":        1,
			"export":      1,
			"webhook":     1,
			"maintenance": 1,
		}
	}
	inner := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: concurrency,
		Queues:      queues,
	})
	return &Server{inner: inner, mux: asynq.NewServeMux()}
}

// Handle registers a HandlerFunc for the given task type. The
// handler receives a driver.Task adapter wrapping the underlying
// *asynq.Task.
//
// Errors wrapping driver.SkipRetry are translated to
// asynq.SkipRetry so the asynq runtime drops the job from the retry
// queue exactly as it did before the driver indirection.
func (s *Server) Handle(taskType string, h driver.HandlerFunc) {
	s.mux.HandleFunc(taskType, func(ctx context.Context, t *asynq.Task) error {
		err := h(ctx, asynqTask{t: t})
		if err != nil && errors.Is(err, driver.SkipRetry) {
			// 既存挙動: handler が SkipRetry を wrap した error を
			// 返すと、asynq の retry を抑止する。元の error message
			// は %w 連鎖でそのまま保持する。
			return fmt.Errorf("%w: %w", err, asynq.SkipRetry)
		}
		return err
	})
}

// Start launches the asynq worker in the background. Returns
// immediately once the worker goroutines are up.
func (s *Server) Start() error { return s.inner.Start(s.mux) }

// Shutdown gracefully stops the worker, waiting for in-flight jobs
// to finish.
func (s *Server) Shutdown() { s.inner.Shutdown() }
