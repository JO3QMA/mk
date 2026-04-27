package queue

import (
	"time"

	"github.com/shiroha-a/mk/internal/queue/driver"
)

// MaintenanceQueueName is the queue used for low-priority maintenance
// jobs (chart tick / resync / clean etc). Kept separate from the
// latency-sensitive deliver queue so a long aggregation does not stall
// federation traffic.
const MaintenanceQueueName = "maintenance"

// Scheduler is the driver-neutral facade over driver.Scheduler. It
// registers cron-style entries once at startup and continuously
// enqueues tasks per their schedule onto the standard worker queues.
type Scheduler struct {
	inner driver.Scheduler
}

// NewScheduler wraps the driver's scheduler.
func NewScheduler(d driver.Driver) *Scheduler {
	return &Scheduler{inner: d.Scheduler()}
}

// RegisterChartJobs registers the 3 chart-related cron jobs.
//
//   - tickCharts : 毎時 55 分 (TS Misskey と同一 cron pattern)
//   - resyncCharts: 毎日 00:00 UTC
//   - cleanCharts : 毎日 00:00 UTC
//
// 重複 enqueue 防止のため Unique TTL を cron 周期と合わせる。前回の
// 同種ジョブが処理中のまま次回 cron が発火しても、TTL 内であれば
// driver が重複 enqueue を弾く。
func (s *Scheduler) RegisterChartJobs() error {
	jobs := []struct {
		cron      string
		taskType  string
		uniqueTTL time.Duration
	}{
		{"55 * * * *", TaskTypeChartTick, 1 * time.Hour},
		{"0 0 * * *", TaskTypeChartResync, 24 * time.Hour},
		{"0 0 * * *", TaskTypeChartClean, 24 * time.Hour},
	}
	for _, j := range jobs {
		if err := s.inner.Register(j.cron, j.taskType, nil,
			driver.WithQueue(MaintenanceQueueName),
			driver.WithMaxRetry(0),
			driver.WithUnique(j.uniqueTTL),
		); err != nil {
			return err
		}
	}
	return nil
}

// RegisterInstanceRefreshJob registers the daily remote-instance metadata
// refresh cron (#393) at 03:00 UTC. The actual walk + fetch is implemented
// by processors.InstanceRefreshProcessor.
func (s *Scheduler) RegisterInstanceRefreshJob() error {
	return s.inner.Register("0 3 * * *", TaskTypeInstanceRefresh, nil,
		driver.WithQueue(MaintenanceQueueName),
		driver.WithMaxRetry(0),
		driver.WithUnique(24*time.Hour),
	)
}

// RegisterRetentionJob registers the daily retention aggregation cron
// (#421) at 00:00 UTC. The actual computation is implemented by
// processors.RetentionAggregateProcessor.
func (s *Scheduler) RegisterRetentionJob() error {
	return s.inner.Register("0 0 * * *", TaskTypeRetentionAggregate, nil,
		driver.WithQueue(MaintenanceQueueName),
		driver.WithMaxRetry(0),
		driver.WithUnique(24*time.Hour),
	)
}

// Start launches the scheduler in the background. Returns immediately.
func (s *Scheduler) Start() error { return s.inner.Start() }

// Shutdown stops the scheduler and releases its connection.
func (s *Scheduler) Shutdown() { s.inner.Shutdown() }
