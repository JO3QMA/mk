package channels

import (
	"encoding/json"
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/stream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotifications_AuthenticatedSubscribesPerUser(t *testing.T) {
	ctx := newCtx(&model.User{ID: "alice"})
	ch := NewNotifications(ctx)
	ch.Init(nil)
	assert.Equal(t, []string{"notifications:alice"}, ctx.subs)

	ch.OnRedisEvent([]byte(`{"id":"n1","type":"follow"}`))
	require.Len(t, ctx.sentType, 1)
	assert.Equal(t, "notification", ctx.sentType[0])

	ch.OnClientMessage("ignored", json.RawMessage(`{}`))
	ch.Dispose()
	assert.Equal(t, []string{"notifications:alice"}, ctx.unsubs)
}

func TestNotifications_AnonymousIsNoOp(t *testing.T) {
	ctx := newCtx(nil)
	ch := NewNotifications(ctx)
	err := ch.Init(nil)
	assert.ErrorIs(t, err, stream.ErrInvalidParams)
	assert.Empty(t, ctx.subs)
	ch.Dispose()
	assert.Empty(t, ctx.unsubs)
}

func TestNotifications_NilUserPointer(t *testing.T) {
	ctx := newCtx((*model.User)(nil))
	ch := NewNotifications(ctx)
	err := ch.Init(nil)
	assert.ErrorIs(t, err, stream.ErrInvalidParams)
	assert.Empty(t, ctx.subs)
}

func TestMain_AuthenticatedSubscribesBoth(t *testing.T) {
	ctx := newCtx(&model.User{ID: "alice"})
	ch := NewMain(ctx)
	ch.Init(nil)
	assert.ElementsMatch(t, []string{"notifications:alice", "main:alice"}, ctx.subs)

	// envelope なし → "notification" として転送される
	ch.OnRedisEvent([]byte(`{"id":"n1"}`))
	require.Len(t, ctx.sentType, 1)
	assert.Equal(t, "notification", ctx.sentType[0])

	// envelope あり → そのタイプとして転送される
	ch.OnRedisEvent([]byte(`{"type":"follow","body":{"userId":"u2"}}`))
	require.Len(t, ctx.sentType, 2)
	assert.Equal(t, "follow", ctx.sentType[1])

	ch.OnClientMessage("ignored", json.RawMessage(`{}`))
	ch.Dispose()
	assert.ElementsMatch(t, []string{"notifications:alice", "main:alice"}, ctx.unsubs)
}

func TestMain_AnonymousIsNoOp(t *testing.T) {
	ctx := newCtx(nil)
	ch := NewMain(ctx)
	err := ch.Init(nil)
	assert.ErrorIs(t, err, stream.ErrInvalidParams)
	assert.Empty(t, ctx.subs)
	ch.Dispose()
	assert.Empty(t, ctx.unsubs)
}

func TestMain_BadJSONEnvelopeFallsBack(t *testing.T) {
	ctx := newCtx(&model.User{ID: "alice"})
	ch := NewMain(ctx)
	ch.Init(nil)
	// invalid JSON は notification にフォールバックして送信される
	ch.OnRedisEvent([]byte(`{not json`))
	require.Len(t, ctx.sentType, 1)
	assert.Equal(t, "notification", ctx.sentType[0])
}
