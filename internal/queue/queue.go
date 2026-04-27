// Package queue is the driver-neutral facade for mk-go's job queue.
// Callers depend on this package (Enqueuer, Server, Inspector,
// Scheduler) without taking a compile-time dependency on the
// underlying queue runtime — that lives behind queue/driver.
//
// AP delivery, webhooks, web push, and maintenance / chart cron
// jobs all flow through this package. Driver swaps (asynq → mkq)
// touch only the wiring code that constructs the driver.Driver in
// internal/server.
package queue

import (
	"context"
	"encoding/json"
	"time"

	"github.com/shiroha-a/mk/internal/queue/driver"
)

// mustMarshal serializes a payload via json.Marshal. mk-go の queue
// payload は string / []byte / 単純な struct のみで構成されている
// ため Marshal は失敗しない。エラー戻り値を呼び出し側に伝播させる
// より panic で wiring バグを早期発見できる方を選んでいる。
func mustMarshal(v any) []byte {
	body, err := json.Marshal(v)
	if err != nil {
		panic("queue: marshal payload: " + err.Error())
	}
	return body
}

// QueueName is the queue used for AP delivery jobs.
const QueueName = "deliver"

// ExportQueueName is the queue for export/import jobs.
const ExportQueueName = "export"

// PushQueueName is the queue for Web Push delivery jobs.
const PushQueueName = "push"

// WebhookQueueName is the queue for user + system webhook delivery jobs.
const WebhookQueueName = "webhook"

// Enqueuer abstracts task enqueueing for callers (DeliverService,
// admin handlers, etc.). The interface is driver-neutral so callers
// can be unit-tested with mocks.
type Enqueuer interface {
	EnqueueDeliver(payload DeliverPayload, opts ...driver.EnqueueOption) error
	EnqueueExport(payload ExportPayload) error
	EnqueueImport(payload ImportPayload) error
	EnqueueWebPush(ctx context.Context, payload WebPushPayload) error
	EnqueueUserWebhook(ctx context.Context, payload WebhookPayload) error
	EnqueueSystemWebhook(ctx context.Context, payload WebhookPayload) error
	Close() error
}

// Client wraps a driver.Client and implements Enqueuer.
type Client struct {
	inner driver.Client
}

// NewClient constructs a Client backed by the supplied driver.
func NewClient(d driver.Driver) *Client {
	return &Client{inner: d.Client()}
}

// EnqueueDeliver puts a deliver task on the queue. opts override the
// default queue selection if they include WithQueue, but normal
// callers should pass payload-only and let the queue routing stay
// fixed.
func (c *Client) EnqueueDeliver(payload DeliverPayload, opts ...driver.EnqueueOption) error {
	body := mustMarshal(payload)
	merged := append([]driver.EnqueueOption{driver.WithQueue(QueueName)}, opts...)
	return c.inner.Enqueue(context.Background(), TaskTypeDeliver, body, merged...)
}

// EnqueueExport puts an export task on the queue.
func (c *Client) EnqueueExport(payload ExportPayload) error {
	body := mustMarshal(payload)
	return c.inner.Enqueue(context.Background(), TaskTypeExport, body,
		driver.WithQueue(ExportQueueName),
	)
}

// EnqueueImport puts an import task on the queue.
func (c *Client) EnqueueImport(payload ImportPayload) error {
	body := mustMarshal(payload)
	return c.inner.Enqueue(context.Background(), TaskTypeImport, body,
		driver.WithQueue(ExportQueueName),
	)
}

// EnqueueImportCustomEmojis puts an admin emoji-zip import task on the
// export queue. Misskey 本家も同じ "dbQueue" (低優先) に積んでいる。
func (c *Client) EnqueueImportCustomEmojis(payload ImportCustomEmojisPayload) error {
	body := mustMarshal(payload)
	return c.inner.Enqueue(context.Background(), TaskTypeImportCustomEmojis, body,
		driver.WithQueue(ExportQueueName),
	)
}

// EnqueueWebPush puts a Web Push delivery task on the push queue.
func (c *Client) EnqueueWebPush(ctx context.Context, payload WebPushPayload) error {
	body := mustMarshal(payload)
	return c.inner.Enqueue(ctx, TaskTypeWebPush, body,
		driver.WithQueue(PushQueueName),
	)
}

// EnqueueUserWebhook puts a user webhook delivery task on the webhook
// queue. Retry policy: 4 attempts (4xx は processor 側で SkipRetry と
// して扱うため実際のリトライ対象は 5xx とネットワークエラーに限られる)。
func (c *Client) EnqueueUserWebhook(ctx context.Context, payload WebhookPayload) error {
	body := mustMarshal(payload)
	return c.inner.Enqueue(ctx, TaskTypeUserWebhook, body,
		driver.WithQueue(WebhookQueueName),
		driver.WithMaxRetry(4),
	)
}

// EnqueueSystemWebhook puts a system webhook delivery task on the webhook queue.
func (c *Client) EnqueueSystemWebhook(ctx context.Context, payload WebhookPayload) error {
	body := mustMarshal(payload)
	return c.inner.Enqueue(ctx, TaskTypeSystemWebhook, body,
		driver.WithQueue(WebhookQueueName),
		driver.WithMaxRetry(4),
	)
}

// EnqueueCleanRemoteNotes puts a remote notes cleaning task on the queue.
// 重複排除のため UniqueFor を設定。
func (c *Client) EnqueueCleanRemoteNotes() error {
	return c.inner.Enqueue(context.Background(), TaskTypeCleanRemoteNotes, nil,
		driver.WithQueue(QueueName),
		driver.WithMaxRetry(0),
		driver.WithUnique(6*time.Hour),
	)
}

// EnqueueReactionFlush puts a reaction flush task on the queue.
func (c *Client) EnqueueReactionFlush() error {
	return c.inner.Enqueue(context.Background(), TaskTypeReactionFlush, nil,
		driver.WithQueue(QueueName),
		driver.WithMaxRetry(0),
		driver.WithUnique(25*time.Second),
	)
}

// EnqueueDeleteAccount schedules a cascade deletion of the user's
// related rows. Uniqueness over a 24h window prevents duplicate jobs
// if the admin clicks delete multiple times while the previous run is
// still processing.
func (c *Client) EnqueueDeleteAccount(payload DeleteAccountPayload) error {
	body := mustMarshal(payload)
	return c.inner.Enqueue(context.Background(), TaskTypeDeleteAccount, body,
		driver.WithQueue(QueueName),
		driver.WithMaxRetry(2),
		driver.WithUnique(24*time.Hour),
	)
}

// Close releases the underlying client connection.
func (c *Client) Close() error { return c.inner.Close() }

// Server is the worker side facade. It registers HandlerFuncs by
// task type and starts/stops the worker loop.
type Server struct {
	inner driver.Server
}

// ServerConfig is kept for backward compatibility with callers that
// pass a Concurrency value via internal/server. The driver itself
// gets its own concrete config (e.g. asynqdriver.ServerConfig)
// at construction time.
type ServerConfig struct {
	Concurrency int
}

// NewServer wraps the driver's Server. The driver must already be
// configured with the desired concurrency / queue weights.
func NewServer(d driver.Driver) *Server {
	return &Server{inner: d.Server()}
}

// Handle registers a handler for the given task type.
func (s *Server) Handle(taskType string, handler driver.HandlerFunc) {
	s.inner.Handle(taskType, handler)
}

// Start launches the worker in the background.
func (s *Server) Start() error { return s.inner.Start() }

// Shutdown gracefully stops the worker, waiting for in-flight jobs to finish.
func (s *Server) Shutdown() { s.inner.Shutdown() }
