package stream

import (
	"context"
	"encoding/json"
	"log/slog"
)

// MainStreamPublisher serializes `{type, body}` envelopes to the Redis topic
// `main:{userID}` which the per-user `main` WebSocket channel subscribes to.
// Used to emit events such as `receiveFollowRequest`, `follow`, `unfollow`,
// `meUpdated` etc. to a single target user.
type MainStreamPublisher struct {
	pub PubSubPublisher
}

// NewMainStreamPublisher constructs a MainStreamPublisher backed by the given
// PubSubPublisher.
func NewMainStreamPublisher(pub PubSubPublisher) *MainStreamPublisher {
	return &MainStreamPublisher{pub: pub}
}

// PublishMainEvent emits an event to `main:{userID}` with a `{type, body}`
// envelope, matching TS `main` stream event shape.
func (p *MainStreamPublisher) PublishMainEvent(userID, eventType string, body any) {
	if p.pub == nil || userID == "" || eventType == "" {
		return
	}
	env := map[string]any{"type": eventType, "body": body}
	raw, err := json.Marshal(env)
	if err != nil {
		slog.Warn("main publisher: marshal failed", "event", eventType, "err", err)
		return
	}
	topic := "main:" + userID
	if err := p.pub.Publish(context.Background(), topic, json.RawMessage(raw)); err != nil {
		slog.Warn("main publisher: publish failed", "topic", topic, "err", err)
	}
}
