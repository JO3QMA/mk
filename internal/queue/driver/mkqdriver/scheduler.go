package mkqdriver

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/shiroha-a/mk/internal/queue/driver"
)

// Scheduler implements driver.Scheduler over mkq's
// UpsertSchedulePattern API. mkq stores schedule registrations as a
// per-queue ZSET; subsequent Register calls with the same scheduleID
// idempotently replace the existing entry (matches asynq.Scheduler's
// "register-once" semantics).
//
// Limitations:
//
//   - mkq's scheduled-fire job inherits its BullMQ Job.name from the
//     queue name, not the scheduled task type. Server.dispatchHandler
//     reads framedPayload.Type instead of Job.name to work around
//     that — admin UI listings still show the queue name in the
//     Job.name column for scheduled fires.
//
//   - mkq's ScheduleOption set (Limit / StartDate / EndDate / TZ /
//     Immediately) does NOT cover per-fire job options like
//     attempts / unique. driver.WithMaxRetry / driver.WithUnique /
//     driver.WithProcessIn passed to Register are therefore silently
//     unsupported. The asynq driver honours all three. Callers that
//     rely on those options for cron jobs should keep them on the
//     ad-hoc Enqueue path or stay on the asynq driver until upstream
//     mkq adds an equivalent.
type Scheduler struct {
	driver *Driver
}

// Register schedules taskType to fire on the given cron pattern. The
// scheduleID is taken from taskType so re-registering the same task
// at a different cron replaces (rather than duplicates) the entry.
//
// driver.WithMaxRetry, driver.WithUnique, and driver.WithProcessIn
// are accepted (so the mkqdriver and asynqdriver share a call site)
// but are NOT honoured — see the Scheduler doc-comment for the
// upstream limitation. A startup warning is emitted when the caller
// passes any of these so the gap is visible in operator logs.
func (s *Scheduler) Register(cronspec, taskType string, payload []byte, opts ...driver.EnqueueOption) error {
	o := driver.ApplyEnqueueOptions(opts)
	if o.Queue == "" {
		return fmt.Errorf("mkqdriver: Scheduler.Register requires WithQueue (taskType=%q)", taskType)
	}
	q := s.driver.queueFor(o.Queue)
	if q == nil {
		return fmt.Errorf("mkqdriver: unknown queue %q (taskType=%q)", o.Queue, taskType)
	}
	if o.MaxRetrySet || o.UniqueTTL > 0 || o.ProcessIn > 0 {
		slog.Warn("mkqdriver: Scheduler.Register dropped per-fire options (mkq scheduler does not propagate them)",
			"taskType", taskType,
			"queue", o.Queue,
			"maxRetrySet", o.MaxRetrySet,
			"uniqueTTL", o.UniqueTTL,
			"processIn", o.ProcessIn,
		)
	}
	framed := framedPayload{Type: taskType, Body: payload}
	return q.UpsertSchedulePattern(context.Background(), taskType, cronspec, framed)
}

// Start is a no-op for mkq — schedules are evaluated lazily by the
// Worker dispatch loop on every prefetch. The method exists to
// satisfy the driver.Scheduler interface.
func (s *Scheduler) Start() error { return nil }

// Shutdown is a no-op for mkq — scheduled entries are persisted in
// Redis and survive driver restarts; nothing to release on the
// scheduler side.
func (s *Scheduler) Shutdown() {}
