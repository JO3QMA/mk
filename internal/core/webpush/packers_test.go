package webpush_test

import (
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/core/webpush"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
)

func TestUserRepoPacker_Found(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	repo.Users["u1"] = &model.User{ID: "u1", Username: "alice", UsernameLower: "alice"}

	p := webpush.NewUserRepoPacker(repo)
	out, ok := p.PackUserByID("u1")
	assert.True(t, ok)
	assert.Equal(t, "alice", out["username"])
}

func TestUserRepoPacker_NotFound(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	p := webpush.NewUserRepoPacker(repo)
	_, ok := p.PackUserByID("missing")
	assert.False(t, ok)
}

func TestUserRepoPacker_NilRepo(t *testing.T) {
	p := webpush.NewUserRepoPacker(nil)
	_, ok := p.PackUserByID("x")
	assert.False(t, ok)
}

func TestUserRepoPacker_NilReceiver(t *testing.T) {
	var p *webpush.UserRepoPacker
	_, ok := p.PackUserByID("x")
	assert.False(t, ok)
}

func TestNoteRepoPacker_NilReceiver(t *testing.T) {
	var p *webpush.NoteRepoPacker
	_, ok := p.PackNoteByID("x")
	assert.False(t, ok)
}

func TestNoteRepoPacker_NilRepo(t *testing.T) {
	idGen, _ := id.NewGenerator("aidx")
	p := webpush.NewNoteRepoPacker(nil, idGen)
	_, ok := p.PackNoteByID("x")
	assert.False(t, ok)
}

func TestNoteRepoPacker_Found(t *testing.T) {
	idGen, _ := id.NewGenerator("aidx")
	repo := testutil.NewMockNoteRepository()
	now := time.Now()
	n := &model.Note{
		ID:         idGen.Generate(now),
		UserID:     "u1",
		Visibility: model.NoteVisibilityPublic,
	}
	text := "hello"
	n.Text = &text
	repo.Notes[n.ID] = n

	p := webpush.NewNoteRepoPacker(repo, idGen)
	out, ok := p.PackNoteByID(n.ID)
	assert.True(t, ok)
	assert.Equal(t, n.ID, out["id"])
	assert.Equal(t, "hello", out["text"])
}

func TestNoteRepoPacker_NotFound(t *testing.T) {
	idGen, _ := id.NewGenerator("aidx")
	repo := testutil.NewMockNoteRepository()
	p := webpush.NewNoteRepoPacker(repo, idGen)
	_, ok := p.PackNoteByID("missing")
	assert.False(t, ok)
}
