package stream

import (
	"context"
	"encoding/json"
	"log/slog"
)

// ReversiGamePublisher serializes {type, body} envelopes to the Redis topic
// `reversiGame:{gameID}` which the reversi WebSocket channel subscribes to.
// 本家 ReversiService の publish パターンと同じ。
type ReversiGamePublisher struct {
	pub PubSubPublisher
}

// NewReversiGamePublisher constructs a ReversiGamePublisher.
func NewReversiGamePublisher(pub PubSubPublisher) *ReversiGamePublisher {
	return &ReversiGamePublisher{pub: pub}
}

// PublishGameEvent implements core/reversi.GamePublisher.
func (p *ReversiGamePublisher) PublishGameEvent(gameID, eventType string, body any) {
	if p.pub == nil || gameID == "" {
		return
	}
	env := map[string]any{"type": eventType, "body": body}
	raw, err := json.Marshal(env)
	if err != nil {
		slog.Warn("reversi publisher: marshal failed", "event", eventType, "err", err)
		return
	}
	topic := "reversiGame:" + gameID
	if err := p.pub.Publish(context.Background(), topic, json.RawMessage(raw)); err != nil {
		slog.Warn("reversi publisher: publish failed", "topic", topic, "err", err)
	}
}
