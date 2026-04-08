package chart

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ManagementService runs the periodic Save() loop that flushes all
// registered charts to the database. The upstream implementation calls
// chart.save() every 20 minutes; we replicate that with a single
// goroutine driven by a time.Ticker.
type ManagementService struct {
	charts   []*Chart
	interval time.Duration
	logger   func(format string, args ...any)

	mu     sync.Mutex
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewManagementService constructs a ManagementService. interval defaults
// to 20 minutes when zero or negative.
func NewManagementService(charts []*Chart, interval time.Duration) *ManagementService {
	if interval <= 0 {
		interval = 20 * time.Minute
	}
	return &ManagementService{
		charts:   charts,
		interval: interval,
		logger:   func(string, ...any) {},
	}
}

// SetLogger installs a printf-style logger for save errors. Defaults to
// a no-op so library users can opt out of logging.
func (m *ManagementService) SetLogger(fn func(format string, args ...any)) {
	if fn != nil {
		m.logger = fn
	}
}

// Start launches the background save loop. Calling Start more than
// once without an intervening Stop returns an error so wiring bugs are
// caught loudly.
func (m *ManagementService) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		return errors.New("chart: management service already started")
	}
	loopCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.wg.Add(1)
	go m.loop(loopCtx)
	return nil
}

// Stop signals the background loop to exit and blocks until it has
// flushed any in-flight Save(). The provided context is honoured for
// the final SaveAll, mirroring the upstream "dispose" hook.
func (m *ManagementService) Stop(ctx context.Context) {
	m.mu.Lock()
	cancel := m.cancel
	m.cancel = nil
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	m.wg.Wait()
	// 終了時にも一度 Save する (本家の dispose と同じ)
	if err := m.SaveAll(ctx); err != nil {
		m.logger("chart: final save failed: %v", err)
	}
}

// SaveAll iterates the registered charts and calls Save() on each.
// Errors are logged and accumulated; the first error is returned so
// callers can decide whether to retry.
func (m *ManagementService) SaveAll(ctx context.Context) error {
	var firstErr error
	for _, c := range m.charts {
		if err := c.Save(ctx); err != nil {
			m.logger("chart: save %s failed: %v", c.Name(), err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (m *ManagementService) loop(ctx context.Context) {
	defer m.wg.Done()
	tick := time.NewTicker(m.interval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if err := m.SaveAll(ctx); err != nil {
				m.logger("chart: periodic save failed: %v", err)
			}
		}
	}
}
