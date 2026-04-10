package queue

import (
	"github.com/hibiken/asynq"
)

// Inspector wraps asynq.Inspector to provide queue management.
type Inspector struct {
	inner *asynq.Inspector
}

// NewInspector creates a new Inspector.
func NewInspector(redisOpt asynq.RedisClientOpt) *Inspector {
	return &Inspector{inner: asynq.NewInspector(redisOpt)}
}

// InspectorInfo holds basic queue statistics.
type InspectorInfo struct {
	Queue     string
	Size      int
	Active    int
	Pending   int
	Completed int
	Failed    int
	Scheduled int
	Retry     int
}

// Queues returns the list of queue names.
func (i *Inspector) Queues() ([]string, error) {
	return i.inner.Queues()
}

// GetQueueInfo returns statistics for a specific queue.
func (i *Inspector) GetQueueInfo(qname string) (*InspectorInfo, error) {
	info, err := i.inner.GetQueueInfo(qname)
	if err != nil {
		return nil, err
	}
	return &InspectorInfo{
		Queue:     info.Queue,
		Size:      info.Size,
		Active:    info.Active,
		Pending:   info.Pending,
		Completed: info.Completed,
		Failed:    info.Failed,
		Scheduled: info.Scheduled,
		Retry:     info.Retry,
	}, nil
}

// Close releases the underlying inspector.
func (i *Inspector) Close() error {
	return i.inner.Close()
}
