package processors

import (
	"context"
	"log/slog"

	"github.com/hibiken/asynq"
)

// RetentionAggregator is the narrow interface the daily retention job needs.
// Matches `core/retention.Service.Aggregate`.
type RetentionAggregator interface {
	Aggregate(ctx context.Context) error
}

// RetentionAggregateProcessor delegates to the retention aggregation
// service on each scheduled fire (#421)。失敗はログだけ残してジョブとしては
// success 扱いにする (asynq の retry に任せて雪崩らせる利点が無い)。
type RetentionAggregateProcessor struct {
	svc RetentionAggregator
}

// NewRetentionAggregateProcessor constructs a processor. nil svc turns the
// handler into a no-op so callers can wire it before the service is ready.
func NewRetentionAggregateProcessor(svc RetentionAggregator) *RetentionAggregateProcessor {
	return &RetentionAggregateProcessor{svc: svc}
}

// Handle implements the asynq handler contract.
func (p *RetentionAggregateProcessor) Handle(ctx context.Context, _ *asynq.Task) error {
	if p.svc == nil {
		return nil
	}
	if err := p.svc.Aggregate(ctx); err != nil {
		slog.Warn("retention aggregate: failed", "err", err)
		return err
	}
	slog.Info("retention aggregate: done")
	return nil
}
