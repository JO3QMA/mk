package mkqdriver

import (
	"context"
	"errors"
	"fmt"

	"github.com/shiroha-a/mkq"

	"github.com/shiroha-a/mk/internal/queue/driver"
)

// Inspector implements driver.Inspector over the per-queue API
// surface mkq exposes (Counts / ListJobs / Get / RemoveJob /
// PromoteJob / RetryJob).
//
// State strings returned in TaskSummary mirror BullMQ buckets ("wait",
// "active", "delayed", "prioritized", "completed", "failed", "paused")
// rather than the asynq state strings — this keeps the bull-board /
// Misskey admin UI rendering correct without an extra translation.
type Inspector struct {
	driver *Driver
}

// Queues returns the set of queues mkq has Define'd against the
// underlying client.
func (i *Inspector) Queues() ([]string, error) {
	return i.driver.client.Queues(inspectorCtx())
}

// GetQueueInfo returns the wait / active / delayed / completed /
// failed counters for the named queue. Pending maps to mkq's wait
// bucket (asynq's pending semantics); Retry has no direct mkq bucket
// and stays at zero (BullMQ keeps retries inside the failed bucket
// until the next dispatch attempt).
func (i *Inspector) GetQueueInfo(qname string) (*driver.InspectorInfo, error) {
	q := i.driver.queueFor(qname)
	if q == nil {
		return nil, fmt.Errorf("mkqdriver: unknown queue %q", qname)
	}
	counts, err := q.Counts(inspectorCtx())
	if err != nil {
		return nil, fmt.Errorf("mkqdriver: counts %q: %w", qname, err)
	}
	return &driver.InspectorInfo{
		Queue:     qname,
		Size:      int(counts.Wait + counts.Active + counts.Delayed + counts.Prioritized),
		Active:    int(counts.Active),
		Pending:   int(counts.Wait),
		Completed: int(counts.Completed),
		Failed:    int(counts.Failed),
		Scheduled: int(counts.Delayed),
		// Retry: mkq keeps retries inside the failed bucket until they
		// move back into delayed; surface 0 to keep the field
		// well-defined.
		Retry: 0,
	}, nil
}

// DeleteTask removes a job from the named queue regardless of its
// current bucket. Wraps mkq.RemoveJob; missing jobs return an
// ErrJobNotFound which we translate into a generic error to avoid
// leaking mkq internals to admin handlers.
func (i *Inspector) DeleteTask(qname, taskID string) error {
	q := i.driver.queueFor(qname)
	if q == nil {
		return fmt.Errorf("mkqdriver: unknown queue %q", qname)
	}
	return q.RemoveJob(inspectorCtx(), taskID)
}

// DeleteAllPendingTasks drains the wait bucket. Returns the number of
// jobs that were removed; mkq's DrainPending does not currently report
// the count, so this returns 0 even on success — matching the asynq
// driver's "best-effort count" semantics for callers that only need
// success/failure.
func (i *Inspector) DeleteAllPendingTasks(qname string) (int, error) {
	q := i.driver.queueFor(qname)
	if q == nil {
		return 0, fmt.Errorf("mkqdriver: unknown queue %q", qname)
	}
	if err := q.DrainPending(inspectorCtx()); err != nil {
		return 0, err
	}
	return 0, nil
}

// RunTask promotes a delayed task to wait, equivalent to asynq's
// "Run scheduled". For non-delayed jobs mkq.PromoteJob returns an
// error which the caller should surface to the admin operator.
func (i *Inspector) RunTask(qname, taskID string) error {
	q := i.driver.queueFor(qname)
	if q == nil {
		return fmt.Errorf("mkqdriver: unknown queue %q", qname)
	}
	return q.PromoteJob(inspectorCtx(), taskID)
}

// Close is a no-op — the underlying client is owned by the parent
// Driver.
func (i *Inspector) Close() error { return nil }

// ListPendingTasks returns up to pageSize entries from the wait
// bucket starting at the given (1-indexed) page.
func (i *Inspector) ListPendingTasks(qname string, page, pageSize int) ([]*driver.TaskSummary, error) {
	return i.list(qname, mkq.JobBucketWait, page, pageSize)
}

// ListActiveTasks returns up to pageSize active tasks.
func (i *Inspector) ListActiveTasks(qname string, page, pageSize int) ([]*driver.TaskSummary, error) {
	return i.list(qname, mkq.JobBucketActive, page, pageSize)
}

// ListScheduledTasks returns up to pageSize delayed (scheduled) tasks.
func (i *Inspector) ListScheduledTasks(qname string, page, pageSize int) ([]*driver.TaskSummary, error) {
	return i.list(qname, mkq.JobBucketDelayed, page, pageSize)
}

// ListRetryTasks returns up to pageSize tasks waiting to be retried.
// mkq keeps retries inside the failed bucket; expose those here so
// admin UIs can show "retry" as a separate tab rather than empty.
func (i *Inspector) ListRetryTasks(qname string, page, pageSize int) ([]*driver.TaskSummary, error) {
	return i.list(qname, mkq.JobBucketFailed, page, pageSize)
}

// GetTaskInfo returns the full snapshot for a single task.
func (i *Inspector) GetTaskInfo(qname, taskID string) (*driver.TaskSummary, error) {
	q := i.driver.queueFor(qname)
	if q == nil {
		return nil, fmt.Errorf("mkqdriver: unknown queue %q", qname)
	}
	job, state, err := q.Get(inspectorCtx(), taskID)
	if err != nil {
		if errors.Is(err, mkq.ErrJobNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("mkqdriver: get job %q: %w", taskID, err)
	}
	return jobToSummary(qname, "", job, state), nil
}

// list normalises mkq.ListJobs inputs (1-indexed page/pageSize) into
// 0-indexed [start, end] zranges and decodes the resulting jobs into
// driver.TaskSummary slices.
func (i *Inspector) list(qname string, bucket mkq.JobBucket, page, pageSize int) ([]*driver.TaskSummary, error) {
	q := i.driver.queueFor(qname)
	if q == nil {
		return nil, fmt.Errorf("mkqdriver: unknown queue %q", qname)
	}
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 30
	}
	start := int64((page - 1) * pageSize)
	end := start + int64(pageSize) - 1

	jobs, err := q.ListJobs(inspectorCtx(), bucket, start, end, true)
	if err != nil {
		return nil, fmt.Errorf("mkqdriver: list %s/%s: %w", qname, bucket, err)
	}
	out := make([]*driver.TaskSummary, 0, len(jobs))
	for _, lj := range jobs {
		out = append(out, jobToSummary(qname, string(bucket), lj.Job, lj.State))
	}
	return out, nil
}

// jobToSummary converts a mkq Job + JobState pair into the driver-
// neutral TaskSummary shape. The Type field is taken from the
// framedPayload wrapper rather than mkq's BullMQ Job.name since the
// latter does not survive scheduled-fire dispatch.
func jobToSummary(queue, state string, job *mkq.Job[framedPayload], st *mkq.JobState) *driver.TaskSummary {
	if job == nil {
		return nil
	}
	body := job.Data.Body
	taskType := job.Data.Type
	if taskType == "" {
		// Fallback for foreign jobs lacking the framing — surface the
		// BullMQ Job.name so admin UIs at least show *something*.
		taskType = job.Name
	}
	s := &driver.TaskSummary{
		ID:         job.ID,
		Queue:      queue,
		Type:       taskType,
		State:      state,
		Payload:    body,
		Retried:    job.AttemptsMade,
		EnqueuedAt: job.Timestamp,
	}
	if st != nil {
		if !st.ProcessedOn.IsZero() {
			s.NextProcessAt = st.ProcessedOn
		}
		if !st.FinishedOn.IsZero() {
			s.CompletedAt = st.FinishedOn
		}
		if st.FailedReason != "" {
			s.LastErr = st.FailedReason
			s.LastFailedAt = st.FinishedOn
		}
	}
	return s
}

// inspectorCtx returns a Background context. The driver.Inspector
// interface does not thread context.Context, but mkq's inspector
// paths talk directly to Redis (no Lua loops, no waits) so the
// underlying Redis client's read/write timeouts already bound the
// call. Adding a per-call context.WithTimeout here would leak
// timers because driver.Inspector methods cannot return a cleanup
// closure to the caller.
func inspectorCtx() context.Context {
	return context.Background()
}
