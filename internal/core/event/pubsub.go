package event

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/redis/go-redis/v9"
)

// PubSubService provides Redis-backed pub/sub messaging.
type PubSubService struct {
	client *redis.Client
	prefix string

	mu   sync.RWMutex
	subs map[string]*redis.PubSub
}

// NewPubSubService creates a new PubSubService.
func NewPubSubService(client *redis.Client, prefix string) *PubSubService {
	return &PubSubService{
		client: client,
		prefix: prefix,
		subs:   make(map[string]*redis.PubSub),
	}
}

func (p *PubSubService) channel(ch string) string {
	return p.prefix + ch
}

// Publish sends a message to a channel. The payload is JSON-encoded.
func (p *PubSubService) Publish(ctx context.Context, channel string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("pubsub marshal: %w", err)
	}
	return p.client.Publish(ctx, p.channel(channel), data).Err()
}

// Subscribe starts listening on a channel and calls handler for each message.
// The subscription runs in a goroutine and can be stopped via Unsubscribe.
func (p *PubSubService) Subscribe(ctx context.Context, channel string, handler func([]byte)) {
	prefixed := p.channel(channel)
	sub := p.client.Subscribe(ctx, prefixed)

	p.mu.Lock()
	p.subs[prefixed] = sub
	p.mu.Unlock()

	go func() {
		ch := sub.Channel()
		for msg := range ch {
			handler([]byte(msg.Payload))
		}
		slog.Debug("pubsub subscription closed", "channel", channel)
	}()
}

// Unsubscribe stops listening on a channel.
func (p *PubSubService) Unsubscribe(channel string) error {
	prefixed := p.channel(channel)

	p.mu.Lock()
	sub, ok := p.subs[prefixed]
	if ok {
		delete(p.subs, prefixed)
	}
	p.mu.Unlock()

	if !ok {
		return nil
	}
	return sub.Close()
}

// Close closes all active subscriptions.
func (p *PubSubService) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for key, sub := range p.subs {
		if err := sub.Close(); err != nil {
			slog.Warn("failed to close pubsub", "channel", key, "error", err)
		}
		delete(p.subs, key)
	}
	return nil
}
