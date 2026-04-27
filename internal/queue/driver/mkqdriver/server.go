package mkqdriver

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sort"
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
//
// Locking model: s.mu protects s.started, s.handlers, and s.workers
// during the registration / shutdown phases. After Start() returns,
// the dispatch fast-path runs **without** taking s.mu — the handler
// snapshot is captured at Start time and frozen for the lifetime of
// the worker. Handle calls after Start panic, so the freeze is
// maintained.
type Server struct {
	driver      *Driver
	concurrency int
	// maxMetricsPoints は mkq.WithJobMetrics の引数。0 だと metrics
	// 書き込みを無効化、正値で BullMQ-spec の metrics LIST に書き込み
	// が走る。
	maxMetricsPoints int

	mu       sync.Mutex
	handlers map[string]driver.HandlerFunc
	workers  []*mkq.Worker
	started  bool
}

// Handle registers a handler for the given task type. Must be called
// before Start; calls after Start panic. The dispatch fast-path
// captures a snapshot of the handlers map at Start, so the registry
// is frozen for the worker's lifetime.
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
// holding s.mu so the in-flight job draining is not blocked — see
// Shutdown for the same pattern.
//
// Concurrency: the lock is released before mkq.Process is called for
// the first queue, so dispatch goroutines spawned by Process do not
// queue up behind the Start loop. This addresses the brief
// dispatch-blocking that occurred when the lock was held across all
// 5 queue creations.
func (s *Server) Start() error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return errors.New("mkqdriver: Start called twice")
	}
	// Snapshot the handlers map and mark started under the lock; the
	// closure passed to mkq.Process closes over the snapshot rather
	// than s, so the dispatch fast-path needs no synchronisation.
	handlersSnapshot := maps.Clone(s.handlers)
	if handlersSnapshot == nil {
		// Clone of empty/nil map → nil; treat the same as empty so
		// the dispatch closure's lookup branch returns the
		// no-handler error consistently.
		handlersSnapshot = map[string]driver.HandlerFunc{}
	}
	s.started = true
	s.mu.Unlock()

	dispatch := newDispatchHandler(handlersSnapshot)

	// Iterate queue names in deterministic order so partial-startup
	// error messages are reproducible across runs (Go map iteration
	// is randomized).
	names := make([]string, 0, len(s.driver.queues))
	for n := range s.driver.queues {
		names = append(names, n)
	}
	sort.Strings(names)

	started := make([]*mkq.Worker, 0, len(names))
	for _, name := range names {
		q := s.driver.queues[name]
		opts := []mkq.WorkerOption{mkq.WithConcurrency(s.concurrency)}
		if s.maxMetricsPoints > 0 {
			// BullMQ-compatible per-queue metrics opt-in。書き込み有効に
			// すると finalize 時に Lua が `bull:<q>:metrics:*` を更新し、
			// admin/job-queue のチャート (frontend が LRANGE する) や
			// Misskey TS frontend の同 dashboard が使える。
			opts = append(opts, mkq.WithJobMetrics(s.maxMetricsPoints))
		}
		w, err := mkq.Process(q, dispatch, opts...)
		if err != nil {
			stopWorkers(started)
			s.mu.Lock()
			s.started = false
			s.mu.Unlock()
			return fmt.Errorf("mkqdriver: start worker for %q: %w", name, err)
		}
		started = append(started, w)
	}

	s.mu.Lock()
	s.workers = started
	s.mu.Unlock()
	return nil
}

// Shutdown stops every worker, waiting for in-flight jobs to finish.
// Calls after the first are no-ops.
//
// We snapshot the worker list under s.mu and release the lock before
// invoking blocking w.Stop calls. dispatchHandler does NOT acquire
// s.mu (snapshot-based dispatch), but Stop is blocking on in-flight
// handlers and we want symmetric behaviour with Start's failure path.
func (s *Server) Shutdown() {
	s.mu.Lock()
	toStop := s.workers
	s.workers = nil
	s.started = false
	s.mu.Unlock()
	stopWorkers(toStop)
}

// stopWorkers calls Stop on each worker sequentially with a
// background context. Must be called WITHOUT holding s.mu — Stop is
// blocking on in-flight handlers and we never want the lock held
// across that wait.
//
// in-flight ジョブが暴走したら mkq 側の lockDuration で自動回収する。
func stopWorkers(workers []*mkq.Worker) {
	for _, w := range workers {
		_ = w.Stop(context.Background())
	}
}

// newDispatchHandler builds the mkq handler closure that demuxes
// incoming jobs to the registered driver.HandlerFunc. The handlers
// map is captured by reference; callers must guarantee it is not
// mutated after the closure is created (Server.Start enforces this
// by snapshotting via maps.Clone before the call).
//
// driver.SkipRetry → mkq.ErrUnrecoverable conversion lives here so
// processors can keep their existing %w-wrap idiom unchanged.
func newDispatchHandler(handlers map[string]driver.HandlerFunc) mkq.Handler[framedPayload] {
	return func(ctx context.Context, job *mkq.Job[framedPayload]) (any, error) {
		taskType := job.Data.Type
		h := handlers[taskType]
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
