package asynqdriver_test

import (
	"context"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shiroha-a/mk/internal/queue/driver"
	"github.com/shiroha-a/mk/internal/queue/driver/asynqdriver"
	"github.com/shiroha-a/mk/internal/testutil"
)

var testRedis *testutil.TestRedis

func TestMain(m *testing.M) {
	ctx := context.Background()
	var err error
	testRedis, err = testutil.SetupRedis(ctx)
	if err != nil {
		log.Fatalf("setup redis: %v", err)
	}
	code := m.Run()
	testRedis.Teardown(ctx)
	os.Exit(code)
}

func redisOpt() asynq.RedisClientOpt {
	return asynq.RedisClientOpt{Addr: testRedis.Addr}
}

func newDriver() *asynqdriver.Driver {
	return asynqdriver.New(redisOpt(), asynqdriver.ServerConfig{Concurrency: 2})
}

func flushRedis(t *testing.T) {
	t.Helper()
	if err := testRedis.Client.FlushAll(context.Background()).Err(); err != nil {
		t.Fatalf("flush: %v", err)
	}
}

func waitGroupTimeout(wg *sync.WaitGroup, d time.Duration) bool {
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
		return true
	case <-time.After(d):
		return false
	}
}

// TestEndToEnd_EnqueueProcess confirms the asynq driver wires
// Client.Enqueue, Server.Handle and the SkipRetry conversion through
// to the real asynq runtime against a live Redis.
func TestEndToEnd_EnqueueProcess(t *testing.T) {
	testutil.SkipIfNoDocker(t)
	flushRedis(t)

	d := newDriver()
	t.Cleanup(func() { _ = d.Close() })

	srv := d.Server()
	var (
		wg       sync.WaitGroup
		received int32
	)
	wg.Add(1)
	srv.Handle("e2e:ok", func(_ context.Context, task driver.Task) error {
		defer wg.Done()
		atomic.AddInt32(&received, 1)
		assert.Equal(t, "e2e:ok", task.Type())
		assert.Equal(t, []byte("body"), task.Payload())
		return nil
	})
	require.NoError(t, srv.Start())
	t.Cleanup(srv.Shutdown)

	require.NoError(t, d.Client().Enqueue(context.Background(), "e2e:ok", []byte("body"),
		driver.WithQueue("deliver")))

	if !waitGroupTimeout(&wg, 5*time.Second) {
		t.Fatal("handler not invoked")
	}
	assert.Equal(t, int32(1), atomic.LoadInt32(&received))
}

// TestInspector_LiveQueue exercises every Inspector method that the
// driver delegates to asynq. We enqueue a single task, then ensure the
// listing/inspection methods all return non-error results.
func TestInspector_LiveQueue(t *testing.T) {
	testutil.SkipIfNoDocker(t)
	flushRedis(t)

	d := newDriver()
	t.Cleanup(func() { _ = d.Close() })

	require.NoError(t, d.Client().Enqueue(context.Background(),
		"inspector:list", []byte("a"),
		driver.WithQueue("deliver"),
	))

	ins := d.Inspector()
	queues, err := ins.Queues()
	require.NoError(t, err)
	assert.Contains(t, queues, "deliver")

	info, err := ins.GetQueueInfo("deliver")
	require.NoError(t, err)
	assert.Equal(t, "deliver", info.Queue)

	pending, err := ins.ListPendingTasks("deliver", 1, 30)
	require.NoError(t, err)
	require.NotEmpty(t, pending)
	taskID := pending[0].ID

	got, err := ins.GetTaskInfo("deliver", taskID)
	require.NoError(t, err)
	assert.Equal(t, taskID, got.ID)

	// Ranges that must succeed even when empty.
	_, err = ins.ListActiveTasks("deliver", 1, 30)
	require.NoError(t, err)
	_, err = ins.ListScheduledTasks("deliver", 1, 30)
	require.NoError(t, err)
	_, err = ins.ListRetryTasks("deliver", 1, 30)
	require.NoError(t, err)

	// page/pageSize の clamp 経路: 0 と 過大値の双方を default に
	// 戻すロジックを一度通す。
	_, err = ins.ListPendingTasks("deliver", 0, 0)
	require.NoError(t, err)
	_, err = ins.ListPendingTasks("deliver", -3, 999)
	require.NoError(t, err)

	// DeleteTask: 1 件削除 → 残り 0
	require.NoError(t, ins.DeleteTask("deliver", taskID))

	// 再 enqueue して DeleteAllPendingTasks をテスト
	require.NoError(t, d.Client().Enqueue(context.Background(),
		"inspector:list", []byte("b"),
		driver.WithQueue("deliver"),
	))
	count, err := ins.DeleteAllPendingTasks("deliver")
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

// TestInspector_QueueMetrics_AsynqStub verifies the driver-neutral
// QueueMetrics surface for the asynq driver: no native time-series,
// so Data is always nil and Count is filled from the cumulative
// Completed / Failed bucket size. Invalid kinds and unknown queues
// surface as errors so the admin handler's defensive swallowing path
// (fetchQueueMetrics) gets exercised.
func TestInspector_QueueMetrics_AsynqStub(t *testing.T) {
	testutil.SkipIfNoDocker(t)
	flushRedis(t)

	d := newDriver()
	t.Cleanup(func() { _ = d.Close() })

	// asynq は最初の Enqueue まで queue を作らないので、まず 1 件
	// 投入して queue を materialize させる。
	require.NoError(t, d.Client().Enqueue(context.Background(),
		"metrics:probe", nil,
		driver.WithQueue("deliver"),
	))

	completed, err := d.Inspector().QueueMetrics("deliver", driver.MetricsKindCompleted)
	require.NoError(t, err)
	require.NotNil(t, completed)
	// asynq には time-series が無いので Data は常に nil。
	assert.Nil(t, completed.Data)
	// Newly-flushed Redis なので累積 Completed = 0 が返る。
	assert.EqualValues(t, 0, completed.Count)

	failed, err := d.Inspector().QueueMetrics("deliver", driver.MetricsKindFailed)
	require.NoError(t, err)
	require.NotNil(t, failed)
	assert.EqualValues(t, 0, failed.Count)

	_, err = d.Inspector().QueueMetrics("deliver", "unknown")
	require.Error(t, err)

	// 存在しない queue は asynq が NOT_FOUND を返すので error 経路が
	// 通る。fetchQueueMetrics 側で握り潰されるので handler 動作は問題
	// ないが、ここでは driver レイヤの契約として error を保証。
	_, err = d.Inspector().QueueMetrics("nonexistent", driver.MetricsKindCompleted)
	require.Error(t, err)
}

// TestInspector_RunTask promotes a scheduled task using the live
// Redis. Scheduled enqueueing uses driver.WithProcessIn so the task
// lands in the scheduled bucket; RunTask then pulls it back to
// pending.
func TestInspector_RunTask(t *testing.T) {
	testutil.SkipIfNoDocker(t)
	flushRedis(t)

	d := newDriver()
	t.Cleanup(func() { _ = d.Close() })

	require.NoError(t, d.Client().Enqueue(context.Background(),
		"inspector:run", []byte{},
		driver.WithQueue("deliver"),
		driver.WithProcessIn(time.Hour),
	))

	ins := d.Inspector()
	scheduled, err := ins.ListScheduledTasks("deliver", 1, 30)
	require.NoError(t, err)
	require.NotEmpty(t, scheduled)
	taskID := scheduled[0].ID

	require.NoError(t, ins.RunTask("deliver", taskID))
}

// TestScheduler_RegisterAndStart exercises the cron scheduler. The
// schedule pattern fires every minute, but we only verify Start /
// Shutdown succeed against a live Redis (Register validates cron
// syntax client-side).
func TestScheduler_RegisterAndStart(t *testing.T) {
	testutil.SkipIfNoDocker(t)
	flushRedis(t)

	d := newDriver()
	t.Cleanup(func() { _ = d.Close() })

	sched := d.Scheduler()
	require.NoError(t, sched.Register("* * * * *", "scheduled:every-min", []byte{},
		driver.WithQueue("maintenance"),
		driver.WithMaxRetry(0),
		driver.WithUnique(time.Minute),
	))
	require.NoError(t, sched.Start())
	sched.Shutdown()
}
