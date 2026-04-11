package activitypub

import (
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRenderer() *Renderer {
	return NewRenderer(NewURLBuilder("https://example.com"))
}

func TestURLBuilder_Helpers(t *testing.T) {
	b := NewURLBuilder("https://example.com")
	assert.Equal(t, "https://example.com/users/u1", b.UserURI("u1"))
	assert.Equal(t, "https://example.com/users/u1/inbox", b.UserInbox("u1"))
	assert.Equal(t, "https://example.com/users/u1/outbox", b.UserOutbox("u1"))
	assert.Equal(t, "https://example.com/users/u1/followers", b.UserFollowers("u1"))
	assert.Equal(t, "https://example.com/users/u1/following", b.UserFollowing("u1"))
	assert.Equal(t, "https://example.com/users/u1#main-key", b.UserKeyURI("u1"))
	assert.Equal(t, "https://example.com/inbox", b.SharedInbox())
	assert.Equal(t, "https://example.com/notes/n1", b.NoteURI("n1"))
	assert.Equal(t, "https://example.com/notes/n1/activity", b.CreateActivityURI("n1"))
	assert.Equal(t, "https://example.com/follows/a/b", b.FollowURI("a", "b"))
}

func TestRenderer_RenderPerson(t *testing.T) {
	r := newRenderer()
	avatar := "https://example.com/avatar.png"
	name := "Alice"
	u := &model.User{
		ID:           "u1",
		Username:     "alice",
		Name:         &name,
		AvatarURL:    &avatar,
		IsLocked:     true,
		IsExplorable: true,
	}
	p := r.RenderPerson(u, "PUBKEY")
	assert.Equal(t, "https://example.com/users/u1", p.ID)
	assert.Equal(t, "Person", p.Type)
	assert.Equal(t, "alice", p.PreferredUsername)
	assert.Equal(t, "Alice", p.Name)
	assert.Equal(t, "PUBKEY", p.PublicKey.PublicKeyPEM)
	assert.True(t, p.ManuallyApproves)
	assert.True(t, p.Discoverable)
	require.NotNil(t, p.Icon)
	assert.Equal(t, avatar, p.Icon.URL)
}

func TestRenderer_RenderPerson_NoOptionalFields(t *testing.T) {
	r := newRenderer()
	u := &model.User{ID: "u1", Username: "alice"}
	p := r.RenderPerson(u, "PUBKEY")
	assert.Empty(t, p.Name)
	assert.Nil(t, p.Icon)
	assert.False(t, p.ManuallyApproves)
}

func newIDGen(t *testing.T) id.Generator {
	t.Helper()
	g, _ := id.NewGenerator("aidx")
	return g
}

func TestRenderer_RenderNote_Public(t *testing.T) {
	r := newRenderer()
	idGen := newIDGen(t)
	noteID := idGen.Generate(time.Now())
	text := "hello"
	cw := "warning"
	n := &model.Note{
		ID:         noteID,
		UserID:     "author",
		Text:       &text,
		CW:         &cw,
		Visibility: model.NoteVisibilityPublic,
	}
	out := r.RenderNote(n, idGen)
	assert.Equal(t, "Note", out.Type)
	assert.Equal(t, "hello", out.Content)
	assert.Equal(t, "warning", out.Summary)
	assert.True(t, out.Sensitive)
	assert.Contains(t, out.To, Public)
	assert.Contains(t, out.CC, "https://example.com/users/author/followers")
}

func TestRenderer_RenderNote_Home(t *testing.T) {
	r := newRenderer()
	idGen := newIDGen(t)
	n := &model.Note{
		ID:         idGen.Generate(time.Now()),
		UserID:     "author",
		Visibility: model.NoteVisibilityHome,
	}
	out := r.RenderNote(n, idGen)
	assert.Contains(t, out.To, "https://example.com/users/author/followers")
	assert.Contains(t, out.CC, Public)
}

func TestRenderer_RenderNote_Followers(t *testing.T) {
	r := newRenderer()
	idGen := newIDGen(t)
	n := &model.Note{
		ID:         idGen.Generate(time.Now()),
		UserID:     "author",
		Visibility: model.NoteVisibilityFollowers,
	}
	out := r.RenderNote(n, idGen)
	assert.Contains(t, out.To, "https://example.com/users/author/followers")
	assert.Empty(t, out.CC)
}

func TestRenderer_RenderNote_Specified(t *testing.T) {
	r := newRenderer()
	idGen := newIDGen(t)
	n := &model.Note{
		ID:             idGen.Generate(time.Now()),
		UserID:         "author",
		Visibility:     model.NoteVisibilitySpecified,
		VisibleUserIDs: pq.StringArray{"u2", "u3"},
	}
	out := r.RenderNote(n, idGen)
	assert.Contains(t, out.To, "https://example.com/users/u2")
	assert.Contains(t, out.To, "https://example.com/users/u3")
}

// stubMentionResolver returns fixed data keyed by userID.
type stubMentionResolver struct {
	entries map[string]struct{ name, uri string }
}

func (s *stubMentionResolver) ResolveMention(userID string) (name, uri string, ok bool) {
	e, exists := s.entries[userID]
	if !exists {
		return "", "", false
	}
	return e.name, e.uri, true
}

func TestRenderer_RenderNote_WithMentions(t *testing.T) {
	r := newRenderer()
	r.SetMentionResolver(&stubMentionResolver{entries: map[string]struct{ name, uri string }{
		"uA": {name: "@alice", uri: "https://example.com/users/uA"},
		"uB": {name: "@bob@remote.example", uri: "https://remote.example/users/bob"},
	}})
	idGen := newIDGen(t)
	n := &model.Note{
		ID:         idGen.Generate(time.Now()),
		UserID:     "author",
		Visibility: model.NoteVisibilityPublic,
		Mentions:   pq.StringArray{"uA", "uB", "unknown"},
	}
	out := r.RenderNote(n, idGen)

	require.Len(t, out.Tag, 2)
	// unknown user is skipped; both known users end up in tag and to.
	tagged := map[string]string{}
	for _, t := range out.Tag {
		m := t.(Mention)
		tagged[m.Href] = m.Name
	}
	assert.Equal(t, "@alice", tagged["https://example.com/users/uA"])
	assert.Equal(t, "@bob@remote.example", tagged["https://remote.example/users/bob"])
	assert.Contains(t, out.To, "https://example.com/users/uA")
	assert.Contains(t, out.To, "https://remote.example/users/bob")
}

func TestRenderer_RenderNote_WithMentions_DuplicateTo(t *testing.T) {
	// When a mention URI is already in `to` (e.g., Specified visibility with the
	// same user targeted), we must not duplicate it.
	r := newRenderer()
	r.SetMentionResolver(&stubMentionResolver{entries: map[string]struct{ name, uri string }{
		"u2": {name: "@bob", uri: "https://example.com/users/u2"},
	}})
	idGen := newIDGen(t)
	n := &model.Note{
		ID:             idGen.Generate(time.Now()),
		UserID:         "author",
		Visibility:     model.NoteVisibilitySpecified,
		VisibleUserIDs: pq.StringArray{"u2"},
		Mentions:       pq.StringArray{"u2"},
	}
	out := r.RenderNote(n, idGen)
	// u2 URI appears only once.
	count := 0
	for _, v := range out.To {
		if v == "https://example.com/users/u2" {
			count++
		}
	}
	assert.Equal(t, 1, count)
}

func TestRenderer_RenderNote_Reply(t *testing.T) {
	r := newRenderer()
	idGen := newIDGen(t)
	parentID := "parent"
	n := &model.Note{
		ID:         idGen.Generate(time.Now()),
		UserID:     "author",
		ReplyID:    &parentID,
		Visibility: model.NoteVisibilityPublic,
	}
	out := r.RenderNote(n, idGen)
	assert.Equal(t, "https://example.com/notes/parent", out.InReplyTo)
}

func TestRenderer_RenderNote_InvalidIDFallback(t *testing.T) {
	r := newRenderer()
	idGen := newIDGen(t)
	n := &model.Note{
		ID:         "not-a-real-id",
		UserID:     "author",
		Visibility: model.NoteVisibilityPublic,
	}
	out := r.RenderNote(n, idGen)
	assert.NotEmpty(t, out.Published) // フォールバックでもpublishedは入る
}

func TestRenderer_RenderCreate(t *testing.T) {
	r := newRenderer()
	idGen := newIDGen(t)
	noteID := idGen.Generate(time.Now())
	n := &model.Note{ID: noteID, UserID: "author", Visibility: model.NoteVisibilityPublic}
	c := r.RenderCreate(n, idGen)
	assert.Equal(t, "Create", c.Type)
	assert.Equal(t, "https://example.com/users/author", c.Actor)
	assert.NotNil(t, c.Object)
}

func TestRenderer_RenderFollow(t *testing.T) {
	r := newRenderer()
	f := r.RenderFollow("alice", "https://remote.example/users/bob")
	assert.Equal(t, "Follow", f.Type)
	assert.Equal(t, "https://example.com/users/alice", f.Actor)
	assert.Equal(t, "https://remote.example/users/bob", f.Object)
}

func TestRenderer_RenderAccept(t *testing.T) {
	r := newRenderer()
	inner := map[string]any{"type": "Follow"}
	a := r.RenderAccept("alice", inner)
	assert.Equal(t, "Accept", a.Type)
	assert.Equal(t, "https://example.com/users/alice", a.Actor)
	assert.NotNil(t, a.Object)
}

func TestStringValue(t *testing.T) {
	s := "x"
	assert.Equal(t, "x", stringValue(&s))
	assert.Equal(t, "", stringValue(nil))
}

func TestRenderer_RenderLike(t *testing.T) {
	r := newRenderer()
	reactor := &model.User{ID: "alice"}
	l := r.RenderLike(reactor, "https://remote.example/notes/n1", "🎉", "https://example.com/likes/l1")
	assert.Equal(t, "Like", l.Type)
	assert.Equal(t, "https://example.com/users/alice", l.Actor)
	assert.Equal(t, "https://remote.example/notes/n1", l.Object)
	assert.Equal(t, "🎉", l.Content)
	assert.Equal(t, "https://example.com/likes/l1", l.ID)
}

func TestRenderer_RenderUndoLike(t *testing.T) {
	r := newRenderer()
	reactor := &model.User{ID: "alice"}
	like := r.RenderLike(reactor, "https://remote.example/notes/n1", "🎉", "https://example.com/likes/l1")
	u := r.RenderUndoLike(reactor, like)
	assert.Equal(t, "Undo", u.Type)
	assert.Equal(t, "https://example.com/users/alice", u.Actor)
	assert.Equal(t, "https://example.com/likes/l1/undo", u.ID)
	require.NotNil(t, u.Object)
}

func TestRenderer_RenderAnnounce(t *testing.T) {
	r := newRenderer()
	renoter := &model.User{ID: "alice"}
	a := r.RenderAnnounce(renoter, "renote1", "https://remote.example/notes/orig")
	assert.Equal(t, "Announce", a.Type)
	assert.Equal(t, "https://example.com/users/alice", a.Actor)
	assert.Equal(t, "https://remote.example/notes/orig", a.Object)
	assert.Equal(t, "https://example.com/notes/renote1/activity", a.ID)
	assert.Contains(t, a.To, Public)
	assert.Contains(t, a.CC, "https://example.com/users/alice/followers")
}

func TestRenderer_RenderUndoAnnounce(t *testing.T) {
	r := newRenderer()
	renoter := &model.User{ID: "alice"}
	announce := r.RenderAnnounce(renoter, "renote1", "https://remote.example/notes/orig")
	u := r.RenderUndoAnnounce(renoter, announce)
	assert.Equal(t, "Undo", u.Type)
	assert.Equal(t, "https://example.com/users/alice", u.Actor)
	assert.Equal(t, announce.ID+"/undo", u.ID)
}

func TestRenderer_RenderDelete(t *testing.T) {
	r := newRenderer()
	author := &model.User{ID: "alice"}
	d := r.RenderDelete(author, "https://example.com/notes/n1")
	assert.Equal(t, "Delete", d.Type)
	assert.Equal(t, "https://example.com/users/alice", d.Actor)
	assert.Equal(t, "https://example.com/notes/n1#Delete", d.ID)
	tomb, ok := d.Object.(Tombstone)
	require.True(t, ok)
	assert.Equal(t, "Tombstone", tomb.Type)
	assert.Equal(t, "https://example.com/notes/n1", tomb.ID)
	assert.Contains(t, d.To, Public)
}
