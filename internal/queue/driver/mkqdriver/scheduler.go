package mkqdriver

import (
	"context"
	"fmt"

	"github.com/shiroha-a/mk/internal/queue/driver"
)

// Scheduler implements driver.Scheduler over mkq's
// UpsertSchedulePattern API. mkq stores schedule registrations as a
// per-queue ZSET; subsequent Register calls with the same scheduleID
// idempotently replace the existing entry (matches asynq.Scheduler's
// "register-once" semantics).
//
// Limitations: mkq's scheduled-fire job currently inherits its
// BullMQ Job.name from the queue name, not the scheduled task type.
// The Server worker dispatch reads the framedPayload.Type field
// rather than Job.name precisely to work around that — admin UI
// listings will still show the queue name in the Job.name column for
// scheduled fires until mkq exposes a per-schedule name override.
type Scheduler struct {
	driver  *Driver
	started bool
}

// Register schedules taskType to fire on the given cron pattern. The
// scheduleID is taken from taskType so re-registering the same task
// at a different cron replaces (rather than duplicates) the entry.
func (s *Scheduler) Register(cronspec, taskType string, payload []byte, opts ...driver.EnqueueOption) error {
	o := driver.ApplyEnqueueOptions(opts)
	if o.Queue == "" {
		return fmt.Errorf("mkqdriver: Scheduler.Register requires WithQueue (taskType=%q)", taskType)
	}
	q := s.driver.queueFor(o.Queue)
	if q == nil {
		return fmt.Errorf("mkqdriver: unknown queue %q (taskType=%q)", o.Queue, taskType)
	}
	framed := framedPayload{Type: taskType, Body: payload}
	return q.UpsertSchedulePattern(context.Background(), taskType, cronspec, framed)
}

// Start is a no-op for mkq — schedules are evaluated lazily by the
// Worker dispatch loop on every prefetch. The method exists to
// satisfy the driver.Scheduler interface.
func (s *Scheduler) Start() error {
	s.started = true
	return nil
}

// Shutdown is a no-op for mkq — scheduled entries are persisted in
// Redis and survive driver restarts; nothing to release on the
// scheduler side.
func (s *Scheduler) Shutdown() {
	s.started = false
}
