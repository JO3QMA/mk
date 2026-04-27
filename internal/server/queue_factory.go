package server

import (
	"context"
	"fmt"
	"time"

	"github.com/shiroha-a/mk/internal/config"
	"github.com/shiroha-a/mk/internal/queue/driver"
	"github.com/shiroha-a/mk/internal/queue/driver/asynqdriver"
	"github.com/shiroha-a/mk/internal/queue/driver/mkqdriver"
)

// buildQueueDriver constructs the queue driver selected by config.
//
// driverName=="mkq" connects to Redis via mkq.NewClient (PING + SCRIPT
// LOAD); failures bubble up so the server fails to start rather than
// silently falling back to asynq. asynq is the historical default and
// does not Dial on construction, so its branch is infallible at this
// layer.
func buildQueueDriver(ctx context.Context, cfg *config.Config) (driver.Driver, error) {
	concurrency := 16
	if cfg.DeliverJobConcurrency != nil && *cfg.DeliverJobConcurrency > 0 {
		concurrency = *cfg.DeliverJobConcurrency
	}

	switch cfg.JobQueueDriver {
	case "mkq":
		dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		return mkqdriver.New(dialCtx, mkqdriver.Config{
			Redis:       mkqdriver.BuildRedisOptions(cfg.RedisForJobQueue),
			Concurrency: concurrency,
		})
	case "asynq", "":
		return asynqdriver.New(
			asynqdriver.BuildRedisOpt(cfg.RedisForJobQueue),
			asynqdriver.ServerConfig{Concurrency: concurrency},
		), nil
	default:
		return nil, fmt.Errorf("server: unknown jobQueueDriver %q", cfg.JobQueueDriver)
	}
}
