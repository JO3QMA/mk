package asynqdriver

import (
	"errors"

	"github.com/hibiken/asynq"

	"github.com/shiroha-a/mk/internal/queue/driver"
)

// Driver bundles the four asynq sub-services into a single
// driver.Driver. Each call to a getter returns the lazily-built
// component so a Driver instance can be created without immediately
// touching Redis.
type Driver struct {
	redisOpt  asynq.RedisClientOpt
	serverCfg ServerConfig

	client    *Client
	server    *Server
	inspector *Inspector
	scheduler *Scheduler
}

// New constructs an asynq-backed driver.Driver. The redisOpt is
// shared by every sub-service; the serverCfg only affects the
// worker side (concurrency / queue priority weights).
func New(redisOpt asynq.RedisClientOpt, serverCfg ServerConfig) *Driver {
	return &Driver{redisOpt: redisOpt, serverCfg: serverCfg}
}

// Client returns the lazily-constructed driver.Client.
func (d *Driver) Client() driver.Client {
	if d.client == nil {
		d.client = NewClient(d.redisOpt)
	}
	return d.client
}

// Server returns the lazily-constructed driver.Server.
func (d *Driver) Server() driver.Server {
	if d.server == nil {
		d.server = NewServer(d.redisOpt, d.serverCfg)
	}
	return d.server
}

// Inspector returns the lazily-constructed driver.Inspector.
func (d *Driver) Inspector() driver.Inspector {
	if d.inspector == nil {
		d.inspector = NewInspector(d.redisOpt)
	}
	return d.inspector
}

// Scheduler returns the lazily-constructed driver.Scheduler.
func (d *Driver) Scheduler() driver.Scheduler {
	if d.scheduler == nil {
		d.scheduler = NewScheduler(d.redisOpt)
	}
	return d.scheduler
}

// Close releases every constructed sub-service. Components that
// have not been built yet are skipped. Errors are aggregated so a
// failure in one Close does not mask the others.
func (d *Driver) Close() error {
	var errs []error
	if d.client != nil {
		if err := d.client.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if d.inspector != nil {
		if err := d.inspector.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	// Server / Scheduler は Shutdown() で停止する。Close 失敗の概念は
	// asynq 側に存在しないため、ここでは触らない (呼び出し側が
	// driver.Server.Shutdown / driver.Scheduler.Shutdown を別途呼ぶ)。
	return errors.Join(errs...)
}
