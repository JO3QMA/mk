package server

import (
	apiadmin "github.com/shiroha-a/mk/internal/api/admin"
	"github.com/shiroha-a/mk/internal/queue"
)

// queueInspectorAdapter adapts queue.Inspector to admin.QueueInspector.
type queueInspectorAdapter struct {
	inner *queue.Inspector
}

func (a *queueInspectorAdapter) Queues() ([]string, error) {
	return a.inner.Queues()
}

func (a *queueInspectorAdapter) GetQueueInfo(qname string) (*apiadmin.QueueInfoResult, error) {
	info, err := a.inner.GetQueueInfo(qname)
	if err != nil {
		return nil, err
	}
	return &apiadmin.QueueInfoResult{
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

func (a *queueInspectorAdapter) DeleteTask(qname, taskID string) error {
	return a.inner.DeleteTask(qname, taskID)
}

func (a *queueInspectorAdapter) DeleteAllPendingTasks(qname string) (int, error) {
	return a.inner.DeleteAllPendingTasks(qname)
}

func (a *queueInspectorAdapter) RunTask(qname, taskID string) error {
	return a.inner.RunTask(qname, taskID)
}

func (a *queueInspectorAdapter) ListPendingTasks(qname string, page, pageSize int) ([]*apiadmin.QueueTaskSummary, error) {
	rows, err := a.inner.ListPendingTasks(qname, page, pageSize)
	return taskSummariesToAdmin(rows), err
}

func (a *queueInspectorAdapter) ListActiveTasks(qname string, page, pageSize int) ([]*apiadmin.QueueTaskSummary, error) {
	rows, err := a.inner.ListActiveTasks(qname, page, pageSize)
	return taskSummariesToAdmin(rows), err
}

func (a *queueInspectorAdapter) ListScheduledTasks(qname string, page, pageSize int) ([]*apiadmin.QueueTaskSummary, error) {
	rows, err := a.inner.ListScheduledTasks(qname, page, pageSize)
	return taskSummariesToAdmin(rows), err
}

func (a *queueInspectorAdapter) ListRetryTasks(qname string, page, pageSize int) ([]*apiadmin.QueueTaskSummary, error) {
	rows, err := a.inner.ListRetryTasks(qname, page, pageSize)
	return taskSummariesToAdmin(rows), err
}

func (a *queueInspectorAdapter) GetTaskInfo(qname, taskID string) (*apiadmin.QueueTaskSummary, error) {
	t, err := a.inner.GetTaskInfo(qname, taskID)
	if err != nil {
		return nil, err
	}
	return taskSummaryToAdmin(t), nil
}

func taskSummariesToAdmin(rows []*queue.TaskSummary) []*apiadmin.QueueTaskSummary {
	out := make([]*apiadmin.QueueTaskSummary, 0, len(rows))
	for _, r := range rows {
		out = append(out, taskSummaryToAdmin(r))
	}
	return out
}

func taskSummaryToAdmin(t *queue.TaskSummary) *apiadmin.QueueTaskSummary {
	if t == nil {
		return nil
	}
	return &apiadmin.QueueTaskSummary{
		ID:            t.ID,
		Queue:         t.Queue,
		Type:          t.Type,
		State:         t.State,
		Payload:       t.Payload,
		Retried:       t.Retried,
		MaxRetry:      t.MaxRetry,
		LastErr:       t.LastErr,
		LastFailedAt:  t.LastFailedAt,
		NextProcessAt: t.NextProcessAt,
		CompletedAt:   t.CompletedAt,
	}
}
