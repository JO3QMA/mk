package stream

import (
	"context"
	"encoding/json"
	"log/slog"
)

// ChatPublisher implements core/chat.StreamingPublisher by serializing
// {type, body} envelopes and pushing them to the Redis pub/sub bus. The
// channel names match upstream Misskey GlobalEventService helpers so that
// existing frontend clients work unchanged.
//
//   - Room: chatRoomStream:{roomId}
//   - Direct message: chatUserStream:{fromUserId}-{toUserId}
//     本家と同じく A→B の会話ビューと B→A のビューは別トピックに publish
//     する。各ユーザーは自分のビュー (chatUserStream:{myId}-{otherId}) を
//     subscribe する。
type ChatPublisher struct {
	pub PubSubPublisher
}

// NewChatPublisher constructs a ChatPublisher.
func NewChatPublisher(pub PubSubPublisher) *ChatPublisher {
	return &ChatPublisher{pub: pub}
}

// PublishUserMessage dispatches a DM event to BOTH direction topics so that
// the sender's and recipient's connected clients both receive it. 本家
// Misskey の createMessageToUser が publishChatUserStream を 2 回呼ぶのと
// 同等。
func (p *ChatPublisher) PublishUserMessage(ctx context.Context, fromUserID, toUserID, eventType string, body any) {
	if p == nil || p.pub == nil || fromUserID == "" || toUserID == "" {
		return
	}
	raw, err := json.Marshal(map[string]any{"type": eventType, "body": body})
	if err != nil {
		slog.Warn("chat publisher: marshal user envelope failed",
			"event", eventType, "err", err)
		return
	}
	senderTopic := "chatUserStream:" + fromUserID + "-" + toUserID
	recipientTopic := "chatUserStream:" + toUserID + "-" + fromUserID
	if err := p.pub.Publish(ctx, senderTopic, json.RawMessage(raw)); err != nil {
		slog.Warn("chat publisher: publish sender topic failed",
			"topic", senderTopic, "err", err)
	}
	if err := p.pub.Publish(ctx, recipientTopic, json.RawMessage(raw)); err != nil {
		slog.Warn("chat publisher: publish recipient topic failed",
			"topic", recipientTopic, "err", err)
	}
}

// PublishRoomMessage dispatches a room event to the single room topic.
func (p *ChatPublisher) PublishRoomMessage(ctx context.Context, roomID, eventType string, body any) {
	if p == nil || p.pub == nil || roomID == "" {
		return
	}
	raw, err := json.Marshal(map[string]any{"type": eventType, "body": body})
	if err != nil {
		slog.Warn("chat publisher: marshal room envelope failed",
			"event", eventType, "err", err)
		return
	}
	topic := "chatRoomStream:" + roomID
	if err := p.pub.Publish(ctx, topic, json.RawMessage(raw)); err != nil {
		slog.Warn("chat publisher: publish room topic failed",
			"topic", topic, "err", err)
	}
}
