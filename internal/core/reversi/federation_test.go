package reversi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAPGame(t *testing.T) {
	g := NewAPGame("session-123")
	assert.Equal(t, "Game", g.Type)
	assert.Equal(t, GameTypeUUID, g.GameTypeUUID)
	assert.Equal(t, "session-123", g.GameState.GameSessionID)
	assert.Empty(t, g.ExtentFlags)
}

func TestRenderInvite(t *testing.T) {
	a := RenderInvite("https://example.com", "sess1", "https://example.com/users/alice", "https://remote.example/users/bob", "2026-01-01T00:00:00Z")
	assert.Equal(t, "Invite", a.Type)
	assert.Contains(t, a.ID, "sess1")
	assert.Equal(t, "https://remote.example/users/bob", a.To)
	game, ok := a.Object.(APGame)
	require.True(t, ok)
	assert.Equal(t, "sess1", game.GameState.GameSessionID)
}

func TestRenderJoin(t *testing.T) {
	a := RenderJoin("https://example.com", "sess1", "https://example.com/users/bob", "https://remote.example/users/alice", "2026-01-01T00:00:00Z")
	assert.Equal(t, "Join", a.Type)
	assert.Equal(t, "https://remote.example/users/alice", a.To)
}

func TestRenderUpdate(t *testing.T) {
	pos := 42
	state := APGameState{
		GameSessionID: "sess1",
		Type:          "putstone",
		Pos:           &pos,
	}
	a := RenderUpdate("https://example.com", "https://example.com/users/alice", "https://remote.example/users/bob", state)
	assert.Equal(t, "Update", a.Type)
	assert.NotEmpty(t, a.ID, "Update activity must carry id (#417 InboxProcessor requirement)")
	game, ok := a.Object.(APGame)
	require.True(t, ok)
	assert.Equal(t, "putstone", game.GameState.Type)
	assert.Equal(t, 42, *game.GameState.Pos)
}

func TestRenderLeave(t *testing.T) {
	a := RenderLeave("https://example.com", "https://example.com/users/alice", "https://remote.example/users/bob", "sess1")
	assert.Equal(t, "Leave", a.Type)
	assert.NotEmpty(t, a.ID, "Leave activity must carry id")
}

func TestRenderUndo(t *testing.T) {
	invite := RenderInvite("https://example.com", "sess1", "actor", "target", "2026-01-01T00:00:00Z")
	undo := RenderUndo("https://example.com", "actor", invite)
	assert.Equal(t, "Undo", undo.Type)
	assert.NotEmpty(t, undo.ID, "Undo activity must carry id")
}

func TestIsReversiGame(t *testing.T) {
	assert.True(t, IsReversiGame(map[string]any{
		"type":           "Game",
		"game_type_uuid": GameTypeUUID,
	}))
	assert.False(t, IsReversiGame(map[string]any{
		"type": "Note",
	}))
	assert.False(t, IsReversiGame(map[string]any{
		"type":           "Game",
		"game_type_uuid": "wrong-uuid",
	}))
}

func TestParseGameState(t *testing.T) {
	obj := map[string]any{
		"game_state": map[string]any{
			"game_session_id": "sess1",
			"type":            "putstone",
			"pos":             float64(42),
			"ready":           true,
			"key":             "noIrregularRules",
			"value":           true,
		},
	}
	state := ParseGameState(obj)
	require.NotNil(t, state)
	assert.Equal(t, "sess1", state.GameSessionID)
	assert.Equal(t, "putstone", state.Type)
	assert.Equal(t, 42, *state.Pos)
	assert.True(t, *state.Ready)
	assert.Equal(t, "noIrregularRules", state.Key)
}

func TestParseGameState_Nil(t *testing.T) {
	assert.Nil(t, ParseGameState(map[string]any{}))
	assert.Nil(t, ParseGameState(map[string]any{"game_state": "invalid"}))
}

func TestGameSessionURI(t *testing.T) {
	got := GameSessionURI("https://example.com", "sess-1")
	assert.Equal(t, "https://example.com/games/"+GameTypeUUID+"/sess-1", got)
}

func TestParseGameSessionURI(t *testing.T) {
	assert.Equal(t, "sess-x", ParseGameSessionURI("https://example.com/games/"+GameTypeUUID+"/sess-x"))
	// reversi UUID と無関係な URI は空文字
	assert.Equal(t, "", ParseGameSessionURI("https://example.com/notes/abc"))
	assert.Equal(t, "", ParseGameSessionURI(""))
}

func TestRenderReversiReaction(t *testing.T) {
	r := RenderReversiReaction("https://local.example", "sess-r",
		"https://local.example/users/alice", "https://remote.example/users/bob", ":fire:")
	assert.Equal(t, "EmojiReaction", r.Type)
	assert.Equal(t, "https://local.example/users/alice", r.Actor)
	assert.Equal(t, "https://local.example/games/"+GameTypeUUID+"/sess-r", r.Object)
	assert.Equal(t, ":fire:", r.Content)
	assert.Equal(t, ":fire:", r.MisskeyReaction)
	assert.Equal(t, "https://remote.example/users/bob", r.To)
}
