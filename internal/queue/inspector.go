package queue

import (
	"time"

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

// DeleteTask deletes a task by queue and ID.
func (i *Inspector) DeleteTask(qname, taskID string) error {
	return i.inner.DeleteTask(qname, taskID)
}

// DeleteAllPendingTasks deletes all pending tasks in a queue.
func (i *Inspector) DeleteAllPendingTasks(qname string) (int, error) {
	return i.inner.DeleteAllPendingTasks(qname)
}

// RunTask promotes a scheduled/retry task to run immediately.
func (i *Inspector) RunTask(qname, taskID string) error {
	return i.inner.RunTask(qname, taskID)
}

// Close releases the underlying inspector.
func (i *Inspector) Close() error {
	return i.inner.Close()
}

// TaskSummary is a lightweight projection of asynq.TaskInfo for admin list APIs.
type TaskSummary struct {
	ID            string
	Queue         string
	Type          string
	State         string
	Payload       []byte
	Retried       int
	MaxRetry      int
	LastErr       string
	LastFailedAt  time.Time
	NextProcessAt time.Time
	EnqueuedAt    time.Time
	ScheduledAt   time.Time
	CompletedAt   time.Time
}

// ListPendingTasks returns up to pageSize pending tasks in qname starting at
// page (1-indexed).
func (i *Inspector) ListPendingTasks(qname string, page, pageSize int) ([]*TaskSummary, error) {
	return i.listTasks(qname, "pending", page, pageSize)
}

// ListActiveTasks returns up to pageSize tasks currently being processed.
func (i *Inspector) ListActiveTasks(qname string, page, pageSize int) ([]*TaskSummary, error) {
	return i.listTasks(qname, "active", page, pageSize)
}

// ListScheduledTasks returns tasks scheduled for a future time.
func (i *Inspector) ListScheduledTasks(qname string, page, pageSize int) ([]*TaskSummary, error) {
	return i.listTasks(qname, "scheduled", page, pageSize)
}

// ListRetryTasks returns tasks waiting to be retried.
func (i *Inspector) ListRetryTasks(qname string, page, pageSize int) ([]*TaskSummary, error) {
	return i.listTasks(qname, "retry", page, pageSize)
}

// GetTaskInfo returns full info for a single task.
func (i *Inspector) GetTaskInfo(qname, taskID string) (*TaskSummary, error) {
	info, err := i.inner.GetTaskInfo(qname, taskID)
	if err != nil {
		return nil, err
	}
	return taskInfoToSummary(info), nil
}

func (i *Inspector) listTasks(qname, state string, page, pageSize int) ([]*TaskSummary, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 30
	}
	opts := []asynq.ListOption{asynq.Page(page), asynq.PageSize(pageSize)}
	var tasks []*asynq.TaskInfo
	var err error
	switch state {
	case "pending":
		tasks, err = i.inner.ListPendingTasks(qname, opts...)
	case "active":
		tasks, err = i.inner.ListActiveTasks(qname, opts...)
	case "scheduled":
		tasks, err = i.inner.ListScheduledTasks(qname, opts...)
	case "retry":
		tasks, err = i.inner.ListRetryTasks(qname, opts...)
	}
	if err != nil {
		return nil, err
	}
	out := make([]*TaskSummary, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, taskInfoToSummary(t))
	}
	return out, nil
}

func taskInfoToSummary(t *asynq.TaskInfo) *TaskSummary {
	return &TaskSummary{
		ID:            t.ID,
		Queue:         t.Queue,
		Type:          t.Type,
		State:         t.State.String(),
		Payload:       t.Payload,
		Retried:       t.Retried,
		MaxRetry:      t.MaxRetry,
		LastErr:       t.LastErr,
		LastFailedAt:  t.LastFailedAt,
		NextProcessAt: t.NextProcessAt,
		CompletedAt:   t.CompletedAt,
	}
}
