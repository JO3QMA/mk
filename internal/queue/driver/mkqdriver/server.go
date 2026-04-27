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
// is allowed to assume the registry is frozen and the dispatch
// fast-path takes s.mu only to read s.handlers (no map mutations
// after Start).
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
//
// Failure mode: if a later mkq.Process fails, the workers spawned so
// far are stopped before bubbling up. Stop is invoked **without**
// holding s.mu so the dispatch handler (which acquires s.mu) can
// drain its in-flight job — see Shutdown for the same pattern.
func (s *Server) Start() error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return errors.New("mkqdriver: Start called twice")
	}
	dispatch := s.dispatchHandler()

	var startErr error
	var startedQueue string
	for _, q := range s.driver.queues {
		w, err := mkq.Process(q, dispatch, mkq.WithConcurrency(s.concurrency))
		if err != nil {
			startErr = fmt.Errorf("mkqdriver: start worker for %q: %w", q.Name(), err)
			startedQueue = q.Name()
			break
		}
		s.workers = append(s.workers, w)
	}
	if startErr != nil {
		// Snapshot the workers we managed to start before failure,
		// then release s.mu so the in-flight handlers can run their
		// dispatch loop to completion while w.Stop drains them.
		toStop := s.workers
		s.workers = nil
		s.started = false
		s.mu.Unlock()
		stopWorkers(toStop)
		_ = startedQueue // referenced via wrapped error
		return startErr
	}
	s.started = true
	s.mu.Unlock()
	return nil
}

// Shutdown stops every worker, waiting for in-flight jobs to finish.
// Calls after the first are no-ops.
//
// We snapshot the worker list under s.mu and release the lock before
// invoking blocking w.Stop calls. dispatchHandler acquires s.mu to
// read the handler map, so holding it during Stop would deadlock:
// Stop waits for in-flight handlers, the in-flight handler waits on
// s.mu, neither makes progress.
func (s *Server) Shutdown() {
	s.mu.Lock()
	toStop := s.workers
	s.workers = nil
	s.started = false
	s.mu.Unlock()
	stopWorkers(toStop)
}

// stopWorkers calls Stop on each worker sequentially with a
// background context. Must be called WITHOUT holding s.mu (Stop is
// blocking and dispatchHandler also acquires s.mu).
//
// in-flight ジョブが暴走したら mkq 側の lockDuration で自動回収する。
func stopWorkers(workers []*mkq.Worker) {
	for _, w := range workers {
		_ = w.Stop(context.Background())
	}
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
