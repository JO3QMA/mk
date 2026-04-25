package processors_test

import (
	"context"
	"errors"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/shiroha-a/mk/internal/queue/processors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubAggregator struct {
	called int
	err    error
}

func (s *stubAggregator) Aggregate(_ context.Context) error {
	s.called++
	return s.err
}

func TestRetentionAggregateProcessor_Handle_Success(t *testing.T) {
	agg := &stubAggregator{}
	p := processors.NewRetentionAggregateProcessor(agg)
	require.NoError(t, p.Handle(context.Background(), asynq.NewTask("x", nil)))
	assert.Equal(t, 1, agg.called)
}

func TestRetentionAggregateProcessor_Handle_PropagatesError(t *testing.T) {
	wantErr := errors.New("db boom")
	agg := &stubAggregator{err: wantErr}
	p := processors.NewRetentionAggregateProcessor(agg)
	err := p.Handle(context.Background(), asynq.NewTask("x", nil))
	assert.ErrorIs(t, err, wantErr, "asynq stats should reflect the underlying failure")
	assert.Equal(t, 1, agg.called)
}

// Nil aggregator → no-op without panic. Callers wire the processor before
// the service is ready in some bootstrap orderings.
func TestRetentionAggregateProcessor_Handle_NilSvcIsNoop(t *testing.T) {
	p := processors.NewRetentionAggregateProcessor(nil)
	require.NoError(t, p.Handle(context.Background(), asynq.NewTask("x", nil)))
}
