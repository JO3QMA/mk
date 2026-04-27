package mkqdriver

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/shiroha-a/mkq"

	"github.com/shiroha-a/mk/internal/queue/driver"
)

// Server runs one mkq.Worker per pre-defined queue, dispatching jobs
// to handlers registered via Handle(taskType, fn). The dispatch keys
// off the framedPayload.Type field rather than mkq's BullMQ Job.name —
// that field is not overridable by mkq's schedule API, so framing the
// payload is the only consistent way to recover the task type for
// both ad-hoc and scheduled jobs.
type Server struct {
	driver      *Driver
	concurrency int

	mu       sync.Mutex
	handlers map[string]driver.HandlerFunc
	workers  []*mkq.Worker
	started  bool
}

// Handle registers a handler for the given task type. Must be called
// before Start; calls after Start panic — the worker dispatch loop
// reads the registry without locking on the hot path.
func (s *Server) Handle(taskType string, h driver.HandlerFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		panic("mkqdriver: Handle called after Start")
	}
	s.handlers[taskType] = h
}

// Start spawns one mkq.Worker per pre-defined queue. Returns once the
// worker goroutines are up.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return errors.New("mkqdriver: Start called twice")
	}
	dispatch := s.dispatchHandler()
	for _, q := range s.driver.queues {
		w, err := mkq.Process(q, dispatch, mkq.WithConcurrency(s.concurrency))
		if err != nil {
			// Stop any workers we already started before bubbling up.
			s.stopWorkersLocked()
			return fmt.Errorf("mkqdriver: start worker for %q: %w", q.Name(), err)
		}
		s.workers = append(s.workers, w)
	}
	s.started = true
	return nil
}

// Shutdown stops every worker, waiting for in-flight jobs to finish.
// Calls after the first are no-ops.
func (s *Server) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopWorkersLocked()
}

// stopWorkersLocked must be called with s.mu held.
func (s *Server) stopWorkersLocked() {
	for _, w := range s.workers {
		// Stop が ctx を取るが、graceful shutdown は呼び出し側 (server.go)
		// から signal で切り出す前提なので background ctx で待つ。
		// in-flight ジョブが暴走したら mkq 側の lockDuration で自動回収。
		_ = w.Stop(context.Background())
	}
	s.workers = nil
	s.started = false
}

// dispatchHandler returns the mkq handler that demuxes incoming jobs
// to the registered driver.HandlerFunc by inspecting framedPayload.
//
// driver.SkipRetry → mkq.ErrUnrecoverable conversion lives here so
// processors can keep their existing %w-wrap idiom unchanged.
func (s *Server) dispatchHandler() mkq.Handler[framedPayload] {
	return func(ctx context.Context, job *mkq.Job[framedPayload]) (any, error) {
		taskType := job.Data.Type
		s.mu.Lock()
		h := s.handlers[taskType]
		s.mu.Unlock()
		if h == nil {
			// 未登録 task type は SkipRetry 相当 (再 enqueue しても処理者が
			// いないので無限ループになる)。
			return nil, fmt.Errorf("mkqdriver: no handler for %q: %w", taskType, mkq.ErrUnrecoverable)
		}
		err := h(ctx, mkqTask{taskType: taskType, payload: job.Data.Body})
		if err != nil && errors.Is(err, driver.SkipRetry) {
			return nil, fmt.Errorf("%w: %w", err, mkq.ErrUnrecoverable)
		}
		return nil, err
	}
}
