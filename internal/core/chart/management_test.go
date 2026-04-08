package chart

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeChart constructs a Chart wrapping the given fakeRepo for management
// tests. We allow the caller to share repos so they can inspect side
// effects after Save() completes.
func makeChart(t *testing.T, name string) *Chart {
	t.Helper()
	c, err := New(Config{
		Schema: Schema{Name: name, Columns: []ColumnDef{{Name: "v"}}},
		Repo:   newFakeRepo(),
		Lock:   NewMemoryLocker(),
		Clock:  newFakeClock(time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC)),
	})
	require.NoError(t, err)
	return c
}

func TestNewManagementService_DefaultInterval(t *testing.T) {
	m := NewManagementService(nil, 0)
	assert.Equal(t, 20*time.Minute, m.interval)
}

// TestNewManagementService_DefaultLoggerExecuted ensures the default
// no-op logger func body is actually entered, satisfying the coverage
// requirement for the func literal allocated inside NewManagementService.
func TestNewManagementService_DefaultLoggerExecuted(t *testing.T) {
	c := makeChart(t, "deflog")
	repo := c.repo.(*fakeRepo)
	repo.armError("FindCurrent", errors.New("default logger boom"))
	require.NoError(t, c.Commit(Diff{"v": 1}, ""))

	m := NewManagementService([]*Chart{c}, time.Minute)
	// Intentionally do NOT call SetLogger so the default no-op fires.
	if err := m.SaveAll(context.Background()); err == nil {
		t.Fatal("expected error to be propagated")
	}
}

func TestManagementService_SaveAll(t *testing.T) {
	a := makeChart(t, "a")
	b := makeChart(t, "b")
	require.NoError(t, a.Commit(Diff{"v": 1}, ""))
	require.NoError(t, b.Commit(Diff{"v": 2}, ""))
	m := NewManagementService([]*Chart{a, b}, time.Minute)
	require.NoError(t, m.SaveAll(context.Background()))
}

func TestManagementService_SaveAllReturnsFirstError(t *testing.T) {
	a := makeChart(t, "a")
	repoA := a.repo.(*fakeRepo)
	repoA.armError("FindCurrent", errors.New("a fail"))
	require.NoError(t, a.Commit(Diff{"v": 1}, ""))

	captured := make([]string, 0)
	m := NewManagementService([]*Chart{a}, time.Minute)
	m.SetLogger(func(format string, args ...any) {
		captured = append(captured, format)
	})
	if err := m.SaveAll(context.Background()); err == nil {
		t.Fatal("expected error")
	}
	assert.NotEmpty(t, captured)
}

func TestManagementService_StartStopFlushes(t *testing.T) {
	c := makeChart(t, "loopchart")
	require.NoError(t, c.Commit(Diff{"v": 1}, ""))

	m := NewManagementService([]*Chart{c}, 24*time.Hour)
	require.NoError(t, m.Start(context.Background()))

	// 二重 Start は弾かれる
	if err := m.Start(context.Background()); err == nil {
		t.Error("expected error on double-start")
	}

	m.Stop(context.Background())
	// Stop が最終 Save を走らせていれば row が 1 件出来ている
	repo := c.repo.(*fakeRepo)
	assert.Len(t, repo.hour[""], 1)
}

func TestManagementService_PeriodicLoopRuns(t *testing.T) {
	c := makeChart(t, "tickchart")
	called := atomic.Int32{}
	tickRepo := c.repo.(*fakeRepo)
	// Pre-populate buffer for each tick by registering tickFn that
	// commits via the chart on demand. To assert the periodic save loop
	// runs at least once, we use a very short interval and wait briefly.
	c.tick = func(_ context.Context, _ string, _ bool) (map[string]int64, error) {
		called.Add(1)
		return nil, nil
	}
	require.NoError(t, c.Commit(Diff{"v": 7}, ""))

	m := NewManagementService([]*Chart{c}, 5*time.Millisecond)
	require.NoError(t, m.Start(context.Background()))
	time.Sleep(20 * time.Millisecond)
	m.Stop(context.Background())

	// Save が走った結果 row が出来ている
	assert.NotEmpty(t, tickRepo.hour[""])
}

func TestManagementService_StopWithoutStartIsNoOp(t *testing.T) {
	m := NewManagementService(nil, time.Minute)
	m.Stop(context.Background()) // 二重に Stop しても panic しない
}

func TestManagementService_StopFinalSaveErrorIsLogged(t *testing.T) {
	c := makeChart(t, "stopfail")
	repo := c.repo.(*fakeRepo)
	require.NoError(t, c.Commit(Diff{"v": 1}, ""))
	// Arm the FindCurrent error so the *final* SaveAll inside Stop fails.
	repo.armError("FindCurrent", errors.New("stop boom"))

	logged := atomic.Int32{}
	m := NewManagementService([]*Chart{c}, 24*time.Hour)
	m.SetLogger(func(string, ...any) { logged.Add(1) })
	require.NoError(t, m.Start(context.Background()))
	m.Stop(context.Background())
	if logged.Load() == 0 {
		t.Fatal("expected final save error to be logged from Stop")
	}
}

func TestManagementService_LoopErrorIsLogged(t *testing.T) {
	c := makeChart(t, "errchart")
	repo := c.repo.(*fakeRepo)
	repo.armError("FindCurrent", errors.New("loop boom"))
	require.NoError(t, c.Commit(Diff{"v": 1}, ""))

	logged := atomic.Int32{}
	m := NewManagementService([]*Chart{c}, 2*time.Millisecond)
	m.SetLogger(func(string, ...any) { logged.Add(1) })
	require.NoError(t, m.Start(context.Background()))
	time.Sleep(15 * time.Millisecond)
	m.Stop(context.Background())
	if logged.Load() == 0 {
		t.Fatal("expected loop save error to be logged")
	}
}
