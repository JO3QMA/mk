// Package queue provides a thin wrapper around asynq for the AP delivery
// pipeline. The wrapper exposes an Enqueuer interface so that callers
// (DeliverService) can be unit-tested with mocks.
package queue

import (
	"context"
	"time"

	"github.com/hibiken/asynq"
)

// QueueName is the asynq queue used for AP delivery jobs.
const QueueName = "deliver"

// ExportQueueName is the asynq queue for export/import jobs.
const ExportQueueName = "export"

// PushQueueName is the asynq queue for Web Push delivery jobs.
const PushQueueName = "push"

// WebhookQueueName is the asynq queue for user + system webhook delivery jobs.
const WebhookQueueName = "webhook"

// Enqueuer abstracts the asynq client for callers that only need to enqueue
// tasks. テスト用にmock可能。
type Enqueuer interface {
	EnqueueDeliver(payload DeliverPayload, opts ...asynq.Option) error
	EnqueueExport(payload ExportPayload) error
	EnqueueImport(payload ImportPayload) error
	EnqueueWebPush(ctx context.Context, payload WebPushPayload) error
	EnqueueUserWebhook(ctx context.Context, payload WebhookPayload) error
	EnqueueSystemWebhook(ctx context.Context, payload WebhookPayload) error
	Close() error
}

// Client wraps asynq.Client and implements Enqueuer.
type Client struct {
	inner *asynq.Client
}

// NewClient constructs a Client backed by Redis. 渡された RedisClientOpt は
// asynqがそのまま使う。
func NewClient(redisOpt asynq.RedisClientOpt) *Client {
	return &Client{inner: asynq.NewClient(redisOpt)}
}

// EnqueueDeliver puts a deliver task on the queue. opts には asynq の通常の
// オプション (MaxRetry, Timeout, ProcessIn 等) を渡せる。Queueは内部で固定。
func (c *Client) EnqueueDeliver(payload DeliverPayload, opts ...asynq.Option) error {
	task := NewDeliverTask(payload)
	// 呼び出し側のオプションを優先しつつ、必ずQueueNameに入れる。
	merged := append([]asynq.Option{asynq.Queue(QueueName)}, opts...)
	if _, err := c.inner.Enqueue(task, merged...); err != nil {
		return err
	}
	return nil
}

// EnqueueExport puts an export task on the queue.
func (c *Client) EnqueueExport(payload ExportPayload) error {
	task := NewExportTask(payload)
	_, err := c.inner.Enqueue(task, asynq.Queue(ExportQueueName))
	return err
}

// EnqueueImport puts an import task on the queue.
func (c *Client) EnqueueImport(payload ImportPayload) error {
	task := NewImportTask(payload)
	_, err := c.inner.Enqueue(task, asynq.Queue(ExportQueueName))
	return err
}

// EnqueueImportCustomEmojis puts an admin emoji-zip import task on the
// export queue. Misskey 本家も同じ "dbQueue" (低優先) に積んでいる。
func (c *Client) EnqueueImportCustomEmojis(payload ImportCustomEmojisPayload) error {
	task := NewImportCustomEmojisTask(payload)
	_, err := c.inner.Enqueue(task, asynq.Queue(ExportQueueName))
	return err
}

// EnqueueWebPush puts a Web Push delivery task on the push queue.
func (c *Client) EnqueueWebPush(ctx context.Context, payload WebPushPayload) error {
	task := NewWebPushTask(payload)
	_, err := c.inner.EnqueueContext(ctx, task, asynq.Queue(PushQueueName))
	return err
}

// EnqueueUserWebhook puts a user webhook delivery task on the webhook queue.
// Retry policy: asynq のデフォルト (25 attempts, exponential backoff) を使う。
// 4xx は processor 側で SkipRetry として扱うため実際のリトライ対象は 5xx と
// ネットワークエラーに限られる。
func (c *Client) EnqueueUserWebhook(ctx context.Context, payload WebhookPayload) error {
	task := NewUserWebhookTask(payload)
	_, err := c.inner.EnqueueContext(ctx, task,
		asynq.Queue(WebhookQueueName),
		asynq.MaxRetry(4),
	)
	return err
}

// EnqueueSystemWebhook puts a system webhook delivery task on the webhook queue.
func (c *Client) EnqueueSystemWebhook(ctx context.Context, payload WebhookPayload) error {
	task := NewSystemWebhookTask(payload)
	_, err := c.inner.EnqueueContext(ctx, task,
		asynq.Queue(WebhookQueueName),
		asynq.MaxRetry(4),
	)
	return err
}

// EnqueueCleanRemoteNotes puts a remote notes cleaning task on the queue.
// ペイロードなし (processor が meta から設定を読む)。重複排除のため UniqueFor を設定。
func (c *Client) EnqueueCleanRemoteNotes() error {
	task := asynq.NewTask(TaskTypeCleanRemoteNotes, nil)
	_, err := c.inner.Enqueue(task,
		asynq.Queue(QueueName),
		asynq.MaxRetry(0),
		asynq.Unique(6*time.Hour),
	)
	return err
}

// EnqueueReactionFlush puts a reaction flush task on the queue.
func (c *Client) EnqueueReactionFlush() error {
	task := asynq.NewTask(TaskTypeReactionFlush, nil)
	_, err := c.inner.Enqueue(task,
		asynq.Queue(QueueName),
		asynq.MaxRetry(0),
		asynq.Unique(25*time.Second),
	)
	return err
}

// Close releases the underlying client connection.
func (c *Client) Close() error {
	return c.inner.Close()
}

// ServerConfig configures the worker process.
type ServerConfig struct {
	// Concurrency is the number of worker goroutines.
	Concurrency int
}

// Server wraps asynq.Server and a ServeMux. ハンドラの登録を行ってから
// Start するスタイル。
type Server struct {
	inner *asynq.Server
	mux   *asynq.ServeMux
}

// NewServer constructs a worker server bound to the given Redis connection
// options.
func NewServer(redisOpt asynq.RedisClientOpt, cfg ServerConfig) *Server {
	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = 16
	}
	inner := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: concurrency,
		// Queue priorities: deliver/push が同重み、export は低優先。
		// map のキーが登録済み queue として扱われ、未登録の queue は
		// processor を実行しないので新しい TaskType 追加時は忘れずに足す。
		Queues: map[string]int{
			QueueName:            1,
			PushQueueName:        1,
			ExportQueueName:      1,
			WebhookQueueName:     1,
			MaintenanceQueueName: 1,
		},
	})
	return &Server{inner: inner, mux: asynq.NewServeMux()}
}

// Handle registers a handler for the given task type.
func (s *Server) Handle(taskType string, handler asynq.HandlerFunc) {
	s.mux.HandleFunc(taskType, handler)
}

// Start launches the worker in the background. asynq.Server.Start は
// goroutineで起動するので呼び出し側はブロックされない。
func (s *Server) Start() error {
	return s.inner.Start(s.mux)
}

// Shutdown gracefully stops the worker, waiting for in-flight jobs to finish.
func (s *Server) Shutdown() {
	s.inner.Shutdown()
}
