package queue

import (
	"github.com/shiroha-a/mk/internal/queue/driver"
)

// InspectorInfo aliases driver.InspectorInfo so callers depend on
// the queue package without pulling driver into their imports.
type InspectorInfo = driver.InspectorInfo

// TaskSummary aliases driver.TaskSummary for the same reason.
type TaskSummary = driver.TaskSummary

// Inspector is the driver-neutral facade over driver.Inspector.
type Inspector struct {
	inner driver.Inspector
}

// NewInspector wraps the driver's inspector.
func NewInspector(d driver.Driver) *Inspector {
	return &Inspector{inner: d.Inspector()}
}

// Queues returns the list of queue names known to the inspector.
func (i *Inspector) Queues() ([]string, error) { return i.inner.Queues() }

// GetQueueInfo returns aggregated counters for the named queue.
func (i *Inspector) GetQueueInfo(qname string) (*InspectorInfo, error) {
	return i.inner.GetQueueInfo(qname)
}

// DeleteTask deletes a task by queue and ID.
func (i *Inspector) DeleteTask(qname, taskID string) error {
	return i.inner.DeleteTask(qname, taskID)
}

// DeleteAllPendingTasks deletes all pending tasks in a queue.
func (i *Inspector) DeleteAllPendingTasks(qname string) (int, error) {
	return i.inner.DeleteAllPendingTasks(qname)
}

// RunTask promotes a scheduled / retry task to run immediately.
func (i *Inspector) RunTask(qname, taskID string) error {
	return i.inner.RunTask(qname, taskID)
}

// Close releases the underlying inspector.
func (i *Inspector) Close() error { return i.inner.Close() }

// ListPendingTasks returns up to pageSize pending tasks in qname,
// starting at the given page (1-indexed).
func (i *Inspector) ListPendingTasks(qname string, page, pageSize int) ([]*TaskSummary, error) {
	return i.inner.ListPendingTasks(qname, page, pageSize)
}

// ListActiveTasks returns up to pageSize active tasks.
func (i *Inspector) ListActiveTasks(qname string, page, pageSize int) ([]*TaskSummary, error) {
	return i.inner.ListActiveTasks(qname, page, pageSize)
}

// ListScheduledTasks returns up to pageSize scheduled tasks.
func (i *Inspector) ListScheduledTasks(qname string, page, pageSize int) ([]*TaskSummary, error) {
	return i.inner.ListScheduledTasks(qname, page, pageSize)
}

// ListRetryTasks returns up to pageSize retry tasks.
func (i *Inspector) ListRetryTasks(qname string, page, pageSize int) ([]*TaskSummary, error) {
	return i.inner.ListRetryTasks(qname, page, pageSize)
}

// GetTaskInfo returns full info for a single task.
func (i *Inspector) GetTaskInfo(qname, taskID string) (*TaskSummary, error) {
	return i.inner.GetTaskInfo(qname, taskID)
}
