package stream

import (
	"context"
)

// ContextSubscriber は core/event.PubSubService が満たす最小サブセット。
// テスト時にスタブ化できるよう interface で定義する。
type ContextSubscriber interface {
	Subscribe(ctx context.Context, channel string, handler func([]byte))
	Unsubscribe(channel string) error
}

// EventPubSubBus adapts a ContextSubscriber (in production: core/event.
// PubSubService) to the stream.PubSubBus interface used by Dispatcher.
// PubSubBus.Subscribe drops the context, so we wrap a background context
// internally; PubSubService.Unsubscribe returns an error which is dropped
// (best-effort) since the bus interface is fire-and-forget.
type EventPubSubBus struct {
	inner ContextSubscriber
}

// NewEventPubSubBus constructs an EventPubSubBus around the given subscriber.
func NewEventPubSubBus(inner ContextSubscriber) *EventPubSubBus {
	return &EventPubSubBus{inner: inner}
}

// Subscribe implements PubSubBus.
func (b *EventPubSubBus) Subscribe(topic string, handler func([]byte)) {
	b.inner.Subscribe(context.Background(), topic, handler)
}

// Unsubscribe implements PubSubBus.
func (b *EventPubSubBus) Unsubscribe(topic string) {
	_ = b.inner.Unsubscribe(topic)
}
