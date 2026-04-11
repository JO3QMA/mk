package stream

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// reuse capturingPubSub from reversi_publisher_test.go (same package)

func TestChatPublisher_PublishUserMessage_BothDirections(t *testing.T) {
	bus := &capturingPubSub{}
	p := NewChatPublisher(bus)
	p.PublishUserMessage(context.Background(), "alice", "bob", "message", map[string]any{"id": "m1"})

	require.Len(t, bus.calls, 2)
	topics := map[string]bool{}
	for _, c := range bus.calls {
		topics[c.topic] = true
	}
	assert.True(t, topics["chatUserStream:alice-bob"])
	assert.True(t, topics["chatUserStream:bob-alice"])

	// verify envelope format
	raw, ok := bus.calls[0].payload.(json.RawMessage)
	require.True(t, ok)
	var env struct {
		Type string `json:"type"`
	}
	require.NoError(t, json.Unmarshal(raw, &env))
	assert.Equal(t, "message", env.Type)
}

func TestChatPublisher_PublishUserMessage_NilPub(t *testing.T) {
	p := NewChatPublisher(nil)
	p.PublishUserMessage(context.Background(), "a", "b", "x", nil) // no panic
}

func TestChatPublisher_PublishUserMessage_EmptyIDs(t *testing.T) {
	bus := &capturingPubSub{}
	p := NewChatPublisher(bus)
	p.PublishUserMessage(context.Background(), "", "b", "x", nil)
	p.PublishUserMessage(context.Background(), "a", "", "x", nil)
	assert.Empty(t, bus.calls)
}

func TestChatPublisher_PublishUserMessage_Error(t *testing.T) {
	bus := &capturingPubSub{err: errors.New("pub boom")}
	p := NewChatPublisher(bus)
	// should silently log and continue; no panic
	p.PublishUserMessage(context.Background(), "a", "b", "msg", nil)
}

func TestChatPublisher_PublishRoomMessage(t *testing.T) {
	bus := &capturingPubSub{}
	p := NewChatPublisher(bus)
	p.PublishRoomMessage(context.Background(), "r1", "message", map[string]any{"id": "m1"})
	require.Len(t, bus.calls, 1)
	assert.Equal(t, "chatRoomStream:r1", bus.calls[0].topic)
}

func TestChatPublisher_PublishRoomMessage_NilPub(t *testing.T) {
	p := NewChatPublisher(nil)
	p.PublishRoomMessage(context.Background(), "r1", "x", nil)
}

func TestChatPublisher_PublishRoomMessage_EmptyRoomID(t *testing.T) {
	bus := &capturingPubSub{}
	p := NewChatPublisher(bus)
	p.PublishRoomMessage(context.Background(), "", "x", nil)
	assert.Empty(t, bus.calls)
}

func TestChatPublisher_PublishRoomMessage_Error(t *testing.T) {
	bus := &capturingPubSub{err: errors.New("pub boom")}
	p := NewChatPublisher(bus)
	p.PublishRoomMessage(context.Background(), "r1", "msg", nil)
}
