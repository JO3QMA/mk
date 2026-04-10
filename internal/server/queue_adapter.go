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
