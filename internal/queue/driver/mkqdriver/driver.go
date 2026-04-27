package mkqdriver

import (
	"context"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"
	"github.com/shiroha-a/mkq"

	"github.com/shiroha-a/mk/internal/queue/driver"
)

// QueueNames is the set of logical queues mkqdriver pre-defines at
// startup. New queue names introduced in mk-go must be added here so
// the corresponding mkq.Queue handle is created and a worker is
// spawned for it. The list mirrors the Queues map asynqdriver Server
// configures.
var QueueNames = []string{
	"deliver",
	"push",
	"export",
	"webhook",
	"maintenance",
}

// Config is the per-driver configuration the constructor consumes.
type Config struct {
	// Redis is forwarded to mkq.Config.Redis. Construct it via
	// BuildRedisOptions.
	Redis redis.UniversalOptions

	// KeyPrefix overrides BullMQ's "bull" default. Empty string keeps
	// the BullMQ default — recommended unless the same Redis hosts
	// multiple BullMQ deployments.
	KeyPrefix string

	// Concurrency is the per-queue worker concurrency. Zero falls
	// back to 16, matching the historical asynq default.
	Concurrency int

	// QueueNames overrides the set of queues to pre-define. Nil/empty
	// keeps the package default (QueueNames).
	QueueNames []string
}

// Driver bundles the Client / Server / Inspector / Scheduler that
// share one *mkq.Client instance. New connects + script-loads against
// Redis; close the Driver to release the connection pool.
type Driver struct {
	client *mkq.Client
	cfg    Config
	queues map[string]*mkq.Queue[framedPayload]

	// Sub-components are constructed lazily on first access.
	mu      sync.Mutex
	dClient *Client
	dServer *Server
	dIns    *Inspector
	dSched  *Scheduler
	closed  bool
}

// New connects to Redis, preloads mkq's vendored Lua scripts, and
// pre-defines the configured queues. The returned Driver owns the
// underlying *mkq.Client and must be Close'd to release resources.
//
// The supplied context bounds the connection setup phase only; once
// New returns, the connection is shared by the driver's sub-services
// for the rest of their lifetime.
func New(ctx context.Context, cfg Config) (*Driver, error) {
	mkqCfg := mkq.Config{Redis: cfg.Redis, KeyPrefix: cfg.KeyPrefix}
	client, err := mkq.NewClient(ctx, mkqCfg)
	if err != nil {
		return nil, fmt.Errorf("mkqdriver: connect: %w", err)
	}

	names := cfg.QueueNames
	if len(names) == 0 {
		names = QueueNames
	}
	queues := make(map[string]*mkq.Queue[framedPayload], len(names))
	for _, n := range names {
		queues[n] = mkq.Define[framedPayload](client, n)
	}

	return &Driver{
		client: client,
		cfg:    cfg,
		queues: queues,
	}, nil
}

// queueFor returns the pre-defined queue for the given name, or nil.
func (d *Driver) queueFor(name string) *mkq.Queue[framedPayload] {
	return d.queues[name]
}

// Client returns the lazily-constructed driver.Client.
func (d *Driver) Client() driver.Client {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.dClient == nil {
		d.dClient = &Client{driver: d}
	}
	return d.dClient
}

// Server returns the lazily-constructed driver.Server.
func (d *Driver) Server() driver.Server {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.dServer == nil {
		concurrency := d.cfg.Concurrency
		if concurrency <= 0 {
			concurrency = 16
		}
		d.dServer = &Server{
			driver:      d,
			concurrency: concurrency,
			handlers:    make(map[string]driver.HandlerFunc),
		}
	}
	return d.dServer
}

// Inspector returns the lazily-constructed driver.Inspector.
func (d *Driver) Inspector() driver.Inspector {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.dIns == nil {
		d.dIns = &Inspector{driver: d}
	}
	return d.dIns
}

// Scheduler returns the lazily-constructed driver.Scheduler.
func (d *Driver) Scheduler() driver.Scheduler {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.dSched == nil {
		d.dSched = &Scheduler{driver: d}
	}
	return d.dSched
}

// Close stops the worker (if started) and releases the underlying
// *mkq.Client. Idempotent: subsequent calls are no-ops, matching the
// asynq driver's contract and tolerating double-close from layered
// shutdown hooks.
func (d *Driver) Close() error {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return nil
	}
	d.closed = true
	srv := d.dServer
	d.mu.Unlock()

	if srv != nil {
		srv.Shutdown()
	}
	if err := d.client.Close(); err != nil {
		return fmt.Errorf("mkqdriver: close client: %w", err)
	}
	return nil
}
