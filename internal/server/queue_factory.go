package server

import (
	"context"
	"fmt"
	"time"

	"github.com/shiroha-a/mk/internal/config"
	"github.com/shiroha-a/mk/internal/queue"
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
//
// `<queue>JobConcurrency` / `<queue>JobPerSec` / `<queue>JobMaxAttempts`
// 系の config は queue_factory が driver Config に流して runtime に反映
// する。mk-go には deliver queue しか実装が無いため、現状有効なのは
// `deliverJob*` のみ。inboxJob* / relationshipJob* は TS-compat 用に
// config 自体は受け付けるが現状 no-op (#495 / docs/configuration.md)。
func buildQueueDriver(ctx context.Context, cfg *config.Config) (driver.Driver, error) {
	totalConcurrency := 16
	if cfg.DeliverJobConcurrency != nil && *cfg.DeliverJobConcurrency > 0 {
		totalConcurrency = *cfg.DeliverJobConcurrency
	}

	queueConcurrency := perQueueConcurrencyFromConfig(cfg)
	queueRateLimits := perQueueRatesFromConfig(cfg)

	switch cfg.JobQueueDriver {
	case "mkq":
		dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		return mkqdriver.New(dialCtx, mkqdriver.Config{
			Redis:            mkqdriver.BuildRedisOptions(cfg.RedisForJobQueue),
			Concurrency:      totalConcurrency,
			QueueConcurrency: queueConcurrency,
			QueueRateLimits:  queueRateLimits,
		})
	case "asynq", "":
		return asynqdriver.New(
			asynqdriver.BuildRedisOpt(cfg.RedisForJobQueue),
			asynqdriver.ServerConfig{
				Concurrency: totalConcurrency,
				RateLimits:  queueRateLimits,
			},
		), nil
	default:
		return nil, fmt.Errorf("server: unknown jobQueueDriver %q", cfg.JobQueueDriver)
	}
}

// perQueueConcurrencyFromConfig flattens the deliver/inbox/relationship
// concurrency knobs into a queue-name → worker-count map. Currently only
// `deliver` is wired (mk-go has no inbox / relationship queue); the other
// two are forwarded but dropped by the driver because there is no matching
// queue handle. Documented in docs/configuration.md.
func perQueueConcurrencyFromConfig(cfg *config.Config) map[string]int {
	out := map[string]int{}
	if cfg.DeliverJobConcurrency != nil && *cfg.DeliverJobConcurrency > 0 {
		out["deliver"] = *cfg.DeliverJobConcurrency
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// perQueueRatesFromConfig builds the queue-name → tasks/sec map applied to
// the driver Server's rate-limiter middleware (asynq) / mkq.WithRateLimit
// (mkq). Only deliver is wired today.
func perQueueRatesFromConfig(cfg *config.Config) map[string]int {
	out := map[string]int{}
	if cfg.DeliverJobPerSec != nil && *cfg.DeliverJobPerSec > 0 {
		out["deliver"] = *cfg.DeliverJobPerSec
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// applyClientPolicies copies enqueue-side defaults (currently
// deliverJobMaxAttempts) onto the queue.Client. Called once at server
// construction so EnqueueDeliver can pre-pend WithMaxRetry when callers
// don't override.
func applyClientPolicies(c *queue.Client, cfg *config.Config) {
	p := queue.Policy{}
	if cfg.DeliverJobMaxAttempts != nil && *cfg.DeliverJobMaxAttempts > 0 {
		p.MaxAttempts = *cfg.DeliverJobMaxAttempts
	}
	if p.MaxAttempts > 0 {
		c.SetPolicy(queue.QueueName, p)
	}
}
