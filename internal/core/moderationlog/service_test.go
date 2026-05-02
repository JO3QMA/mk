package moderationlog

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type spyRepo struct {
	mu   sync.Mutex
	logs []*model.ModerationLog
	err  error
}

func (s *spyRepo) Create(log *model.ModerationLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.logs = append(s.logs, log)
	return nil
}

func (s *spyRepo) List(int, int) ([]*model.ModerationLog, error) { return nil, nil }

func (s *spyRepo) snapshot() []*model.ModerationLog {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*model.ModerationLog, len(s.logs))
	copy(out, s.logs)
	return out
}

func newSvc(t *testing.T, repo *spyRepo) *Service {
	t.Helper()
	gen, err := id.NewGenerator("aidx")
	require.NoError(t, err)
	return New(repo, gen)
}

func TestService_Log_writesEntry(t *testing.T) {
	repo := &spyRepo{}
	svc := newSvc(t, repo)

	svc.Log("actor1", LogResetPassword, map[string]any{
		"userId":       "target1",
		"userUsername": "alice",
		"userHost":     nil,
	})

	require.Eventually(t, func() bool {
		return len(repo.snapshot()) == 1
	}, 2*time.Second, 5*time.Millisecond, "log row should appear")

	logs := repo.snapshot()
	assert.Equal(t, "actor1", logs[0].UserID)
	assert.Equal(t, "resetPassword", logs[0].Type)
	assert.NotEmpty(t, logs[0].ID)

	var info map[string]any
	require.NoError(t, json.Unmarshal(logs[0].Info, &info))
	assert.Equal(t, "target1", info["userId"])
	assert.Equal(t, "alice", info["userUsername"])
}

func TestService_Log_nilInfoEncodesAsEmptyObject(t *testing.T) {
	repo := &spyRepo{}
	svc := newSvc(t, repo)

	svc.Log("actor1", LogSuspend, nil)
	require.Eventually(t, func() bool { return len(repo.snapshot()) == 1 }, 2*time.Second, 5*time.Millisecond)

	assert.JSONEq(t, "{}", string(repo.snapshot()[0].Info))
}

func TestService_Log_nilReceiverIsNoop(t *testing.T) {
	var svc *Service
	// 落ちないだけで十分。fire-and-forget なので返り値はない。
	svc.Log("actor", LogSuspend, nil)
}

func TestService_Log_nilDepsAreNoop(t *testing.T) {
	svc := New(nil, nil)
	svc.Log("actor", LogSuspend, nil)
	// no panic, no assertion needed
}

func TestService_Log_repoErrorDoesNotPanic(t *testing.T) {
	repo := &spyRepo{err: errors.New("db down")}
	svc := newSvc(t, repo)

	// Log は fire-and-forget なので即返る。goroutine 内で slog.Warn される。
	svc.Log("actor1", LogSuspend, map[string]any{"userId": "x"})

	// give the goroutine time to run; it must not panic
	time.Sleep(50 * time.Millisecond)
}

func TestService_Log_unmarshalableInfoIsDropped(t *testing.T) {
	// channel は encoding/json では marshal できないので、Marshal が
	// error を返す path に入って row は書かれずに早期 return する。
	repo := &spyRepo{}
	svc := newSvc(t, repo)

	svc.Log("actor1", LogSuspend, map[string]any{
		"bad": make(chan int),
	})

	// allow any goroutine to settle
	time.Sleep(20 * time.Millisecond)
	assert.Empty(t, repo.snapshot(), "unmarshalable info should be dropped, not persisted")
}

// panicRepo's Create panics so we can verify safeGo's recover keeps the
// goroutine from taking down the test process.
type panicRepo struct{}

func (panicRepo) Create(*model.ModerationLog) error {
	panic("boom")
}
func (panicRepo) List(int, int) ([]*model.ModerationLog, error) { return nil, nil }

func TestService_Log_recoverFromGoroutinePanic(t *testing.T) {
	gen, err := id.NewGenerator("aidx")
	require.NoError(t, err)
	svc := New(panicRepo{}, gen)

	// 何が起きても (panic 含む) Log 呼び出し自体は即返る。
	svc.Log("actor1", LogSuspend, map[string]any{"userId": "x"})

	// goroutine が panic しても test process は落ちない (recover 経由)。
	time.Sleep(50 * time.Millisecond)
}
