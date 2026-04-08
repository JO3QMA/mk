package stream

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubContextSubscriber implements ContextSubscriber for tests.
type stubContextSubscriber struct {
	subTopics   []string
	unsubTopics []string
	unsubErr    error
}

func (s *stubContextSubscriber) Subscribe(_ context.Context, channel string, _ func([]byte)) {
	s.subTopics = append(s.subTopics, channel)
}

func (s *stubContextSubscriber) Unsubscribe(channel string) error {
	s.unsubTopics = append(s.unsubTopics, channel)
	return s.unsubErr
}

func TestEventPubSubBus_SubscribeForwardsToInner(t *testing.T) {
	inner := &stubContextSubscriber{}
	bus := NewEventPubSubBus(inner)
	bus.Subscribe("home:alice", func([]byte) {})
	require.Len(t, inner.subTopics, 1)
	assert.Equal(t, "home:alice", inner.subTopics[0])
}

func TestEventPubSubBus_UnsubscribeForwardsToInner(t *testing.T) {
	inner := &stubContextSubscriber{}
	bus := NewEventPubSubBus(inner)
	bus.Unsubscribe("home:alice")
	require.Len(t, inner.unsubTopics, 1)
	assert.Equal(t, "home:alice", inner.unsubTopics[0])
}

func TestEventPubSubBus_UnsubscribeIgnoresInnerError(t *testing.T) {
	inner := &stubContextSubscriber{unsubErr: errors.New("oops")}
	bus := NewEventPubSubBus(inner)
	// best-effort: error is swallowed, no panic
	bus.Unsubscribe("topic")
	assert.Len(t, inner.unsubTopics, 1)
}
