package entity

import (
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/core/notification"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestPackNotification_NilInput(t *testing.T) {
	assert.Nil(t, PackNotification(nil, nil, nil, nil))
}

func TestPackNotification_FollowType_WithUser(t *testing.T) {
	idGen, _ := id.NewGenerator("aidx")
	nid := idGen.Generate(time.Now())
	n := &notification.Notification{
		ID:         nid,
		CreatedAt:  time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC),
		Type:       notification.TypeFollow,
		NotifierID: "u_bob",
	}
	user := &model.User{ID: "u_bob", Username: "bob", UsernameLower: "bob"}
	out := PackNotification(n, user, nil, idGen)
	assert.Equal(t, nid, out["id"])
	assert.Equal(t, "2026-04-19T12:00:00.000Z", out["createdAt"])
	assert.Equal(t, "follow", out["type"])
	// userId はnotifierIdから取る (TS互換)
	assert.Equal(t, "u_bob", out["userId"])
	// user は packed UserLite
	u, ok := out["user"].(UserLite)
	assert.True(t, ok)
	assert.Equal(t, "bob", u.Username)
	// note はfollowタイプでは無い
	_, hasNote := out["note"]
	assert.False(t, hasNote)
}

func TestPackNotification_ReactionType_WithNote(t *testing.T) {
	idGen, _ := id.NewGenerator("aidx")
	nid := idGen.Generate(time.Now())
	noteID := idGen.Generate(time.Now())
	n := &notification.Notification{
		ID:         nid,
		CreatedAt:  time.Now(),
		Type:       notification.TypeReaction,
		NotifierID: "u_alice",
		NoteID:     noteID,
		Reaction:   ":smile:",
	}
	user := &model.User{ID: "u_alice", Username: "alice"}
	note := &model.Note{ID: noteID, UserID: "u_me", Text: ptrStr("hi")}
	out := PackNotification(n, user, note, idGen)
	assert.Equal(t, "reaction", out["type"])
	assert.Equal(t, "u_alice", out["userId"])
	assert.Equal(t, ":smile:", out["reaction"])
	// note は packed で返る
	_, hasNote := out["note"]
	assert.True(t, hasNote)
}

func TestPackNotification_PollVote_WithChoice(t *testing.T) {
	idGen, _ := id.NewGenerator("aidx")
	choice := 2
	n := &notification.Notification{
		ID:         "x",
		CreatedAt:  time.Now(),
		Type:       notification.TypePollVote,
		NotifierID: "u_voter",
		NoteID:     "n1",
		Choice:     &choice,
	}
	out := PackNotification(n, nil, nil, idGen)
	assert.Equal(t, 2, out["choice"])
	// user / note は nil なので含まれない
	_, hasUser := out["user"]
	assert.False(t, hasUser)
	_, hasNote := out["note"]
	assert.False(t, hasNote)
}

func TestPackNotification_ExtraFields_Merged(t *testing.T) {
	idGen, _ := id.NewGenerator("aidx")
	n := &notification.Notification{
		ID:        "x",
		CreatedAt: time.Now(),
		Type:      notification.TypeExportCompleted,
		Extra:     map[string]any{"fileId": "df_1"},
	}
	out := PackNotification(n, nil, nil, idGen)
	assert.Equal(t, "df_1", out["fileId"])
}

func ptrStr(s string) *string { return &s }
