package queue

import (
	"github.com/hibiken/asynq"
)

// MaintenanceQueueName is the asynq queue used for low-priority
// maintenance jobs (chart tick / resync / clean etc). Kept separate
// from the latency-sensitive deliver queue so a long aggregation does
// not stall federation traffic.
const MaintenanceQueueName = "maintenance"

// Scheduler wraps asynq.Scheduler. It registers cron-style entries
// once at startup and continuously enqueues tasks per their schedule
// onto the standard worker queues.
//
// asynq.Scheduler は redis 上のロックで重複起動を防ぐので、複数 process
// で起動しても同時刻に同じジョブを 2 回 enqueue することはない。
type Scheduler struct {
	inner *asynq.Scheduler
}

// NewScheduler constructs a Scheduler backed by the given Redis.
func NewScheduler(redisOpt asynq.RedisClientOpt) *Scheduler {
	return &Scheduler{
		inner: asynq.NewScheduler(redisOpt, &asynq.SchedulerOpts{}),
	}
}

// RegisterChartJobs registers the 3 chart-related cron jobs.
//
//   - tickCharts : 毎時 55 分 (TS Misskey と同一 cron pattern)
//   - resyncCharts: 毎日 00:00 UTC
//   - cleanCharts : 毎日 00:00 UTC
//
// 重複 enqueue 防止のため Unique TTL を cron 周期と合わせる。
func (s *Scheduler) RegisterChartJobs() error {
	jobs := []struct {
		cron     string
		taskType string
	}{
		{"55 * * * *", TaskTypeChartTick},
		{"0 0 * * *", TaskTypeChartResync},
		{"0 0 * * *", TaskTypeChartClean},
	}
	for _, j := range jobs {
		task := asynq.NewTask(j.taskType, nil)
		if _, err := s.inner.Register(j.cron, task,
			asynq.Queue(MaintenanceQueueName),
			asynq.MaxRetry(0),
		); err != nil {
			return err
		}
	}
	return nil
}

// Start launches the scheduler in the background. Returns immediately.
func (s *Scheduler) Start() error {
	return s.inner.Start()
}

// Shutdown stops the scheduler and releases its Redis connection.
func (s *Scheduler) Shutdown() {
	s.inner.Shutdown()
}
