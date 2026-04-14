package queue

import (
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewScheduler_NotNil is a simple smoke test that the Scheduler
// wrapper builds without touching Redis. asynq.Scheduler does not
// connect on construction, so this can run without a Redis instance.
func TestNewScheduler_NotNil(t *testing.T) {
	s := NewScheduler(asynq.RedisClientOpt{Addr: "127.0.0.1:6379"})
	require.NotNil(t, s)
	require.NotNil(t, s.inner)
	// Shutdown を呼んでも Start していなければ no-op で安全。
	s.Shutdown()
}

// TestScheduler_RegisterChartJobs_NoErr verifies the cron syntax + queue
// options are accepted by asynq. Register does not enqueue — it simply
// records the schedule — so no Redis is required.
func TestScheduler_RegisterChartJobs_NoErr(t *testing.T) {
	s := NewScheduler(asynq.RedisClientOpt{Addr: "127.0.0.1:6379"})
	defer s.Shutdown()

	require.NoError(t, s.RegisterChartJobs())
}

func TestMaintenanceQueueName_Const(t *testing.T) {
	// 既存定数と被らないこと (ベタ書きの同期ミスを防ぐ smoke)
	assert.Equal(t, "maintenance", MaintenanceQueueName)
}
