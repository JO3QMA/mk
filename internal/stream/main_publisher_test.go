package stream

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMainStreamPublisher_PublishEnvelope(t *testing.T) {
	bus := &capturingPubSub{}
	p := NewMainStreamPublisher(bus)
	p.PublishMainEvent("u1", "receiveFollowRequest", map[string]any{"id": "u2", "username": "bob"})

	require.Len(t, bus.calls, 1)
	assert.Equal(t, "main:u1", bus.calls[0].topic)

	raw, ok := bus.calls[0].payload.(json.RawMessage)
	require.True(t, ok)
	var env struct {
		Type string          `json:"type"`
		Body json.RawMessage `json:"body"`
	}
	require.NoError(t, json.Unmarshal(raw, &env))
	assert.Equal(t, "receiveFollowRequest", env.Type)

	var body map[string]any
	require.NoError(t, json.Unmarshal(env.Body, &body))
	assert.Equal(t, "u2", body["id"])
	assert.Equal(t, "bob", body["username"])
}

func TestMainStreamPublisher_NilPub(t *testing.T) {
	p := NewMainStreamPublisher(nil)
	p.PublishMainEvent("u1", "any", nil) // no panic
}

func TestMainStreamPublisher_EmptyUserID(t *testing.T) {
	bus := &capturingPubSub{}
	p := NewMainStreamPublisher(bus)
	p.PublishMainEvent("", "receiveFollowRequest", nil)
	assert.Empty(t, bus.calls)
}

func TestMainStreamPublisher_EmptyEventType(t *testing.T) {
	bus := &capturingPubSub{}
	p := NewMainStreamPublisher(bus)
	p.PublishMainEvent("u1", "", nil)
	assert.Empty(t, bus.calls)
}

func TestMainStreamPublisher_PublishError(t *testing.T) {
	bus := &capturingPubSub{err: errors.New("pub boom")}
	p := NewMainStreamPublisher(bus)
	p.PublishMainEvent("u1", "receiveFollowRequest", nil) // logged but not panicked
}
