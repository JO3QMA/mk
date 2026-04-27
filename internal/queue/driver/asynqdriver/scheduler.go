package asynqdriver

import (
	"github.com/hibiken/asynq"

	"github.com/shiroha-a/mk/internal/queue/driver"
)

// Scheduler implements driver.Scheduler over *asynq.Scheduler.
type Scheduler struct {
	inner *asynq.Scheduler
}

// NewScheduler constructs a Scheduler bound to the given Redis.
//
// asynq.Scheduler は redis 上のロックで重複起動を防ぐので、複数 process
// で起動しても同時刻に同じジョブを 2 回 enqueue することはない。
func NewScheduler(redisOpt asynq.RedisClientOpt) *Scheduler {
	return &Scheduler{
		inner: asynq.NewScheduler(redisOpt, &asynq.SchedulerOpts{}),
	}
}

// Register schedules the named task to fire on every cron tick.
// Options follow the EnqueueOption set; ProcessIn is ignored (cron
// already controls when the task fires).
func (s *Scheduler) Register(cronspec, taskType string, payload []byte, opts ...driver.EnqueueOption) error {
	o := driver.ApplyEnqueueOptions(opts)
	task := asynq.NewTask(taskType, payload)
	asynqOpts := toAsynqOptions(o)
	_, err := s.inner.Register(cronspec, task, asynqOpts...)
	return err
}

// Start launches the scheduler in the background. Returns immediately.
func (s *Scheduler) Start() error { return s.inner.Start() }

// Shutdown stops the scheduler and releases its Redis connection.
func (s *Scheduler) Shutdown() { s.inner.Shutdown() }
