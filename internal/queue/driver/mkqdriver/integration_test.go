package mkqdriver_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shiroha-a/mk/internal/queue/driver"
	"github.com/shiroha-a/mk/internal/queue/driver/mkqdriver"
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

func newDriver(t *testing.T) *mkqdriver.Driver {
	t.Helper()
	testutil.SkipIfNoDocker(t)
	flushRedis(t)
	d, err := mkqdriver.New(context.Background(), mkqdriver.Config{
		Redis:       redis.UniversalOptions{Addrs: []string{testRedis.Addr}},
		Concurrency: 4,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	return d
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

// TestEndToEnd_EnqueueProcess submits a single job, runs the worker
// against the live Redis, and verifies the dispatch handler observed
// the original task type and payload.
func TestEndToEnd_EnqueueProcess(t *testing.T) {
	d := newDriver(t)

	srv := d.Server()
	var (
		wg       sync.WaitGroup
		received int32
		gotType  string
		gotBody  []byte
		mu       sync.Mutex
	)
	wg.Add(1)
	srv.Handle("test:ok", func(_ context.Context, task driver.Task) error {
		defer wg.Done()
		atomic.AddInt32(&received, 1)
		mu.Lock()
		gotType = task.Type()
		gotBody = task.Payload()
		mu.Unlock()
		return nil
	})
	require.NoError(t, srv.Start())
	t.Cleanup(srv.Shutdown)

	require.NoError(t, d.Client().Enqueue(context.Background(), "test:ok", []byte(`{"a":1}`),
		driver.WithQueue("deliver")))

	if !waitGroupTimeout(&wg, 5*time.Second) {
		t.Fatal("handler not invoked within timeout")
	}
	assert.Equal(t, int32(1), atomic.LoadInt32(&received))
	mu.Lock()
	assert.Equal(t, "test:ok", gotType)
	assert.Equal(t, []byte(`{"a":1}`), gotBody)
	mu.Unlock()
}

// TestEnqueue_UnknownQueueRejects ensures the driver refuses to fall
// back to a default queue for callers that forget WithQueue.
func TestEnqueue_UnknownQueueRejects(t *testing.T) {
	d := newDriver(t)
	err := d.Client().Enqueue(context.Background(), "x", nil,
		driver.WithQueue("not-a-queue"))
	require.Error(t, err)

	err = d.Client().Enqueue(context.Background(), "x", nil)
	require.Error(t, err)
}

// TestEnqueue_DuplicateUniqueDropsSilently confirms WithUnique TTL
// matches asynq's silent-drop behaviour.
func TestEnqueue_DuplicateUniqueDropsSilently(t *testing.T) {
	d := newDriver(t)
	for i := 0; i < 3; i++ {
		require.NoError(t, d.Client().Enqueue(context.Background(),
			"unique:test", nil,
			driver.WithQueue("deliver"),
			driver.WithUnique(time.Minute),
		))
	}
	// Counts should be 1 wait job. Inspector exercises Counts().
	info, err := d.Inspector().GetQueueInfo("deliver")
	require.NoError(t, err)
	assert.Equal(t, 1, info.Pending)
}

// TestServer_HandleSkipRetryConvertsToUnrecoverable ensures the
// driver-level SkipRetry sentinel reaches mkq as ErrUnrecoverable
// (which mkq surfaces as a permanent failure rather than a retry).
func TestServer_HandleSkipRetryConvertsToUnrecoverable(t *testing.T) {
	d := newDriver(t)
	srv := d.Server()
	var wg sync.WaitGroup
	wg.Add(1)
	srv.Handle("test:skip", func(_ context.Context, _ driver.Task) error {
		defer wg.Done()
		return fmt.Errorf("decode boom: %w", driver.SkipRetry)
	})
	require.NoError(t, srv.Start())
	t.Cleanup(srv.Shutdown)

	require.NoError(t, d.Client().Enqueue(context.Background(), "test:skip", nil,
		driver.WithQueue("deliver")))
	if !waitGroupTimeout(&wg, 5*time.Second) {
		t.Fatal("handler not invoked")
	}

	// Wait for finalisation. mkq processes the result asynchronously;
	// the failed counter takes a moment to update after the handler
	// returns.
	require.Eventually(t, func() bool {
		info, err := d.Inspector().GetQueueInfo("deliver")
		if err != nil {
			return false
		}
		return info.Failed >= 1
	}, 5*time.Second, 50*time.Millisecond, "expected job to land in failed bucket")
}

// TestServer_HandleNoRegisteredHandler verifies an unknown job name
// surfaces as a permanent failure (no infinite retry loop).
func TestServer_HandleNoRegisteredHandler(t *testing.T) {
	d := newDriver(t)
	srv := d.Server()
	require.NoError(t, srv.Start())
	t.Cleanup(srv.Shutdown)

	require.NoError(t, d.Client().Enqueue(context.Background(), "not-registered", nil,
		driver.WithQueue("deliver")))

	require.Eventually(t, func() bool {
		info, err := d.Inspector().GetQueueInfo("deliver")
		if err != nil {
			return false
		}
		return info.Failed >= 1
	}, 5*time.Second, 50*time.Millisecond)
}

// TestInspector_FullSurface walks every Inspector entry point against
// the live Redis. Some methods (DrainPending) drop counts to zero;
// each case asserts the post-condition rather than the exact return
// value.
func TestInspector_FullSurface(t *testing.T) {
	d := newDriver(t)

	// Seed the deliver queue with two pending tasks (one ad-hoc, one
	// that we'll later inspect by ID).
	require.NoError(t, d.Client().Enqueue(context.Background(), "ins:a", []byte("a"),
		driver.WithQueue("deliver")))
	require.NoError(t, d.Client().Enqueue(context.Background(), "ins:b", []byte("b"),
		driver.WithQueue("deliver")))

	ins := d.Inspector()

	queues, err := ins.Queues()
	require.NoError(t, err)
	assert.Contains(t, queues, "deliver")

	info, err := ins.GetQueueInfo("deliver")
	require.NoError(t, err)
	assert.Equal(t, 2, info.Pending)

	// Listing
	pending, err := ins.ListPendingTasks("deliver", 1, 30)
	require.NoError(t, err)
	require.Len(t, pending, 2)
	taskID := pending[0].ID
	assert.NotEmpty(t, pending[0].Type)

	got, err := ins.GetTaskInfo("deliver", taskID)
	require.NoError(t, err)
	assert.Equal(t, taskID, got.ID)

	_, err = ins.ListActiveTasks("deliver", 1, 30)
	require.NoError(t, err)
	_, err = ins.ListScheduledTasks("deliver", 1, 30)
	require.NoError(t, err)
	_, err = ins.ListRetryTasks("deliver", 1, 30)
	require.NoError(t, err)

	// Page / pageSize clamp paths.
	_, err = ins.ListPendingTasks("deliver", 0, 0)
	require.NoError(t, err)
	_, err = ins.ListPendingTasks("deliver", -1, 999)
	require.NoError(t, err)

	// Delete one specific task.
	require.NoError(t, ins.DeleteTask("deliver", taskID))
	infoAfterDelete, err := ins.GetQueueInfo("deliver")
	require.NoError(t, err)
	assert.Equal(t, 1, infoAfterDelete.Pending)

	// Drain remaining pending.
	_, err = ins.DeleteAllPendingTasks("deliver")
	require.NoError(t, err)
	infoAfterDrain, err := ins.GetQueueInfo("deliver")
	require.NoError(t, err)
	assert.Equal(t, 0, infoAfterDrain.Pending)

	// Unknown queue returns error.
	_, err = ins.GetQueueInfo("missing")
	require.Error(t, err)
	require.Error(t, ins.DeleteTask("missing", "x"))
	_, err = ins.DeleteAllPendingTasks("missing")
	require.Error(t, err)
	require.Error(t, ins.RunTask("missing", "x"))
	_, err = ins.GetTaskInfo("missing", "x")
	require.Error(t, err)
	_, err = ins.ListPendingTasks("missing", 1, 10)
	require.Error(t, err)
}

// TestInspector_GetQueueInfo_IncludesRepeatSchedules verifies the
// mk-go-specific behaviour added for #455: registering a repeatable
// schedule via Scheduler.Register populates `bull:<queue>:repeat`,
// and Inspector.GetQueueInfo surfaces those entries through the
// Scheduled field even though mkq does not pre-allocate concrete
// delayed jobs into the `bull:<queue>:delayed` ZSET.
//
// admin/job-queue.vue maps Scheduled into its "Delayed" KPI column,
// so without this addition operators running mk-go on the mkq driver
// would see Delayed=0 even with N cron jobs registered.
func TestInspector_GetQueueInfo_IncludesRepeatSchedules(t *testing.T) {
	d := newDriver(t)
	sched := d.Scheduler()

	// Pre-condition: zero before any schedule is registered.
	infoBefore, err := d.Inspector().GetQueueInfo("maintenance")
	require.NoError(t, err)
	assert.Equal(t, 0, infoBefore.Scheduled)

	require.NoError(t, sched.Register("0 0 * * *", "task:daily", nil,
		driver.WithQueue("maintenance"),
	))
	require.NoError(t, sched.Register("*/5 * * * *", "task:every5", nil,
		driver.WithQueue("maintenance"),
	))

	infoAfter, err := d.Inspector().GetQueueInfo("maintenance")
	require.NoError(t, err)
	// addJobScheduler-11.lua は repeat ZSET と delayed ZSET の両方に
	// 書き込むので、Scheduled には少なくとも 2 (repeat ZCARD 由来) と、
	// fresh state では加えて concrete delayed の 2 が足される。本番の
	// steady-state では concrete delayed が即 promote されて 0 になり、
	// repeat ZCARD だけが残る。assertion は「register した数以上が
	// 見える」という性質に留め、過剰カウント側は許容する。
	assert.GreaterOrEqual(t, infoAfter.Scheduled, 2,
		"Scheduled should reflect at least the repeat ZSET cardinality")
	assert.GreaterOrEqual(t, infoAfter.Size, 2)
}

// TestInspector_RunTaskPromotesDelayed verifies that RunTask pulls a
// scheduled task back to wait, mirroring asynq's "Run scheduled".
func TestInspector_RunTaskPromotesDelayed(t *testing.T) {
	d := newDriver(t)

	require.NoError(t, d.Client().Enqueue(context.Background(),
		"ins:run", nil,
		driver.WithQueue("deliver"),
		driver.WithProcessIn(time.Hour),
	))

	ins := d.Inspector()
	scheduled, err := ins.ListScheduledTasks("deliver", 1, 30)
	require.NoError(t, err)
	require.NotEmpty(t, scheduled)

	require.NoError(t, ins.RunTask("deliver", scheduled[0].ID))
	// After promotion the task should sit in pending.
	require.Eventually(t, func() bool {
		info, err := ins.GetQueueInfo("deliver")
		return err == nil && info.Pending >= 1
	}, 3*time.Second, 50*time.Millisecond)
}

// TestScheduler_RegisterRoundtrip exercises the cron Register API and
// the validator branches Scheduler exposes.
func TestScheduler_RegisterRoundtrip(t *testing.T) {
	d := newDriver(t)
	sched := d.Scheduler()
	require.NoError(t, sched.Register("0 0 * * *", "task:daily", nil,
		driver.WithQueue("maintenance"),
		driver.WithMaxRetry(0),
	))
	require.NoError(t, sched.Start())
	sched.Shutdown()

	// Unknown queue → error.
	require.Error(t, sched.Register("0 0 * * *", "task:x", nil,
		driver.WithQueue("missing")))

	// Missing queue → error.
	require.Error(t, sched.Register("0 0 * * *", "task:y", nil))
}

// TestServer_StartTwiceRejects ensures the server cannot be started
// repeatedly — Process holds Redis connections per worker slot and
// re-Start would leak them.
func TestServer_StartTwiceRejects(t *testing.T) {
	d := newDriver(t)
	srv := d.Server()
	require.NoError(t, srv.Start())
	t.Cleanup(srv.Shutdown)

	require.Error(t, srv.Start(), "second Start must be rejected")
}

// TestServer_HandleAfterStartPanics validates the documented contract:
// callers must register every handler before Start.
func TestServer_HandleAfterStartPanics(t *testing.T) {
	d := newDriver(t)
	srv := d.Server()
	require.NoError(t, srv.Start())
	t.Cleanup(srv.Shutdown)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Handle after Start must panic")
		}
	}()
	srv.Handle("late", func(context.Context, driver.Task) error { return nil })
}

// TestDriver_CloseWithoutStart confirms a freshly-built driver cleans
// up cleanly when no sub-services were exercised.
func TestDriver_CloseWithoutStart(t *testing.T) {
	d := newDriver(t)
	// reset Cleanup so we can call Close manually.
	require.NoError(t, d.Close())

	// Close after Close — second call should still succeed (idempotent
	// on the worker side; the underlying redis client returns nil for
	// repeated Close).
	require.NoError(t, d.Close())
}

// TestNewDriver_BadAddressFails ensures the constructor surfaces
// connect failures rather than returning a half-built Driver.
func TestNewDriver_BadAddressFails(t *testing.T) {
	testutil.SkipIfNoDocker(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := mkqdriver.New(ctx, mkqdriver.Config{
		Redis: redis.UniversalOptions{Addrs: []string{"127.0.0.1:1"}},
	})
	require.Error(t, err)
	if !errors.Is(err, context.DeadlineExceeded) && err == nil {
		t.Fatalf("expected non-nil error, got %v", err)
	}
}
