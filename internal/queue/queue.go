// Package queue provides a thin wrapper around asynq for the AP delivery
// pipeline. The wrapper exposes an Enqueuer interface so that callers
// (DeliverService) can be unit-tested with mocks.
package queue

import (
	"github.com/hibiken/asynq"
)

// QueueName is the asynq queue used for AP delivery jobs.
const QueueName = "deliver"

// ExportQueueName is the asynq queue for export/import jobs.
const ExportQueueName = "export"

// Enqueuer abstracts the asynq client for callers that only need to enqueue
// tasks. テスト用にmock可能。
type Enqueuer interface {
	EnqueueDeliver(payload DeliverPayload, opts ...asynq.Option) error
	EnqueueExport(payload ExportPayload) error
	EnqueueImport(payload ImportPayload) error
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
		Queues:      map[string]int{QueueName: 1},
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
