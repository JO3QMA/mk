package asynqdriver

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/shiroha-a/mk/internal/queue/driver"
)

// Client implements driver.Client over *asynq.Client.
type Client struct {
	inner *asynq.Client
}

// NewClient constructs a Client backed by the supplied asynq
// connection options.
func NewClient(redisOpt asynq.RedisClientOpt) *Client {
	return &Client{inner: asynq.NewClient(redisOpt)}
}

// Enqueue submits the given task. driver.WithProcessIn(d) maps to
// asynq.ProcessIn(d); other options follow the obvious mapping.
func (c *Client) Enqueue(ctx context.Context, taskType string, payload []byte, opts ...driver.EnqueueOption) error {
	o := driver.ApplyEnqueueOptions(opts)
	task := asynq.NewTask(taskType, payload)
	asynqOpts := toAsynqOptions(o)
	_, err := c.inner.EnqueueContext(ctx, task, asynqOpts...)
	return err
}

// Close releases the underlying asynq client.
func (c *Client) Close() error { return c.inner.Close() }
