package activitypub

import (
	"time"

	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
)

// URLBuilder constructs canonical URLs for the local instance.
// 同じデータでも複数の場所から参照されるので、ヘルパとして集約しておく。
type URLBuilder struct {
	baseURL string
}

// NewURLBuilder constructs a URLBuilder rooted at baseURL (e.g. "https://example.com").
func NewURLBuilder(baseURL string) *URLBuilder {
	return &URLBuilder{baseURL: baseURL}
}

// UserURI returns the canonical actor URI for a local user.
func (b *URLBuilder) UserURI(userID string) string {
	return b.baseURL + "/users/" + userID
}

// UserInbox returns the inbox URL for a user.
func (b *URLBuilder) UserInbox(userID string) string {
	return b.UserURI(userID) + "/inbox"
}

// UserOutbox returns the outbox URL for a user.
func (b *URLBuilder) UserOutbox(userID string) string {
	return b.UserURI(userID) + "/outbox"
}

// UserFollowers returns the followers collection URL.
func (b *URLBuilder) UserFollowers(userID string) string {
	return b.UserURI(userID) + "/followers"
}

// UserFollowing returns the following collection URL.
func (b *URLBuilder) UserFollowing(userID string) string {
	return b.UserURI(userID) + "/following"
}

// UserKeyURI returns the public key fragment URI used in HTTP signatures.
func (b *URLBuilder) UserKeyURI(userID string) string {
	return b.UserURI(userID) + "#main-key"
}

// SharedInbox returns the shared inbox URL.
func (b *URLBuilder) SharedInbox() string {
	return b.baseURL + "/inbox"
}

// NoteURI returns the canonical note URI.
func (b *URLBuilder) NoteURI(noteID string) string {
	return b.baseURL + "/notes/" + noteID
}

// CreateActivityURI returns the URI of the Create activity wrapping a note.
func (b *URLBuilder) CreateActivityURI(noteID string) string {
	return b.NoteURI(noteID) + "/activity"
}

// FollowURI returns the URI for a Follow activity (followerID-followeeID).
func (b *URLBuilder) FollowURI(followerID, followeeID string) string {
	return b.baseURL + "/follows/" + followerID + "/" + followeeID
}

// MentionResolver resolves a note.Mentions entry (user ID) into the data
// required to build an AS Mention tag. 実装は server/router.go 側で
// UserRepository を wrap する形で提供される。解決に失敗したら ok=false を
// 返す (例: ユーザー削除済み、ID 不整合など)。
//
// uri はローカルユーザーなら urls.UserURI(user.ID)、リモートユーザーなら
// user.URI を返す。name は Misskey 互換で "@username" / "@username@host"。
type MentionResolver interface {
	ResolveMention(userID string) (name, uri string, ok bool)
}

// Renderer converts model entities into AS objects.
type Renderer struct {
	urls            *URLBuilder
	mentionResolver MentionResolver
}

// NewRenderer constructs a Renderer.
func NewRenderer(urls *URLBuilder) *Renderer {
	return &Renderer{urls: urls}
}

// SetMentionResolver attaches a MentionResolver used by RenderNote to populate
// the `tag` field and additional `to` audience entries. nil で無効化できる。
func (r *Renderer) SetMentionResolver(mr MentionResolver) {
	r.mentionResolver = mr
}

// RenderPerson packs a local user into a Person actor object.
// publicKeyPEM はリポジトリから取得した公開鍵PEM文字列。
func (r *Renderer) RenderPerson(u *model.User, publicKeyPEM string) *Person {
	uri := r.urls.UserURI(u.ID)
	p := &Person{
		Object: Object{
			ID:   uri,
			Type: "Person",
			Name: stringValue(u.Name),
		},
		Inbox:             r.urls.UserInbox(u.ID),
		Outbox:            r.urls.UserOutbox(u.ID),
		Followers:         r.urls.UserFollowers(u.ID),
		Following:         r.urls.UserFollowing(u.ID),
		PreferredUsername: u.Username,
		URL:               uri,
		Endpoints:         Endpoints{SharedInbox: r.urls.SharedInbox()},
		PublicKey: PublicKey{
			ID:           r.urls.UserKeyURI(u.ID),
			Owner:        uri,
			PublicKeyPEM: publicKeyPEM,
		},
		ManuallyApproves: u.IsLocked,
		Discoverable:     u.IsExplorable,
	}
	if u.AvatarURL != nil {
		p.Icon = &Image{Type: "Image", URL: *u.AvatarURL}
	}
	AddContext(p)
	return p
}

// RenderNote packs a local note into a Note object.
func (r *Renderer) RenderNote(n *model.Note, idGen id.Generator) *Note {
	out := &Note{
		Object: Object{
			ID:   r.urls.NoteURI(n.ID),
			Type: "Note",
		},
		AttributedTo: r.urls.UserURI(n.UserID),
		Content:      stringValue(n.Text),
		Published:    parseNoteTime(n.ID, idGen),
		Sensitive:    n.CW != nil && *n.CW != "",
	}
	if n.CW != nil {
		out.Summary = *n.CW
	}
	if n.ReplyID != nil {
		out.InReplyTo = r.urls.NoteURI(*n.ReplyID)
	}

	to, cc := r.addressing(n)

	// Resolve mentions into Tag entries and add mentioned user URIs to `to`
	// so receiving instances (特に Misskey) が mention 通知を作成できる。
	// mentionResolver 未設定なら tag は付けない (テスト互換)。
	if r.mentionResolver != nil && len(n.Mentions) > 0 {
		seenTo := make(map[string]struct{}, len(to))
		for _, v := range to {
			seenTo[v] = struct{}{}
		}
		for _, uid := range n.Mentions {
			name, uri, ok := r.mentionResolver.ResolveMention(uid)
			if !ok || uri == "" {
				continue
			}
			out.Tag = append(out.Tag, NewMention(uri, name))
			if _, dup := seenTo[uri]; !dup {
				to = append(to, uri)
				seenTo[uri] = struct{}{}
			}
		}
	}

	out.To = to
	out.CC = cc

	AddContext(out)
	return out
}

// RenderCreate wraps a Note into a Create activity addressed to the same audience.
func (r *Renderer) RenderCreate(n *model.Note, idGen id.Generator) *Create {
	note := r.RenderNote(n, idGen)
	c := &Create{
		Activity: Activity{
			Object: Object{
				ID:   r.urls.CreateActivityURI(n.ID),
				Type: "Create",
			},
			Actor:     r.urls.UserURI(n.UserID),
			Published: note.Published,
			To:        note.To,
			CC:        note.CC,
		},
		Object: note,
	}
	AddContext(c)
	return c
}

// RenderFollow returns a Follow activity from follower → followeeURI.
func (r *Renderer) RenderFollow(followerID, followeeURI string) *Follow {
	f := &Follow{
		Activity: Activity{
			Object: Object{
				ID:   r.urls.FollowURI(followerID, followeeURI),
				Type: "Follow",
			},
			Actor: r.urls.UserURI(followerID),
		},
		Object: followeeURI,
	}
	AddContext(f)
	return f
}

// RenderAccept wraps an inner activity in an Accept.
func (r *Renderer) RenderAccept(actorID string, inner any) *Accept {
	a := &Accept{
		Activity: Activity{
			Object: Object{Type: "Accept"},
			Actor:  r.urls.UserURI(actorID),
		},
		Object: inner,
	}
	AddContext(a)
	return a
}

// RenderLike returns a Like activity for the given reaction.
// targetURI は対象ノートの canonical URI (リモートなら note.URI、ローカルなら
// urls.NoteURI(note.ID))。
func (r *Renderer) RenderLike(reactor *model.User, targetURI string, reaction string, likeID string) *Like {
	l := &Like{
		Activity: Activity{
			Object: Object{
				ID:   likeID,
				Type: "Like",
			},
			Actor: r.urls.UserURI(reactor.ID),
		},
		Object:  targetURI,
		Content: reaction,
	}
	AddContext(l)
	return l
}

// RenderUndoLike wraps a previously emitted Like in an Undo activity.
func (r *Renderer) RenderUndoLike(reactor *model.User, like *Like) *Undo {
	u := &Undo{
		Activity: Activity{
			Object: Object{
				ID:   like.ID + "/undo",
				Type: "Undo",
			},
			Actor: r.urls.UserURI(reactor.ID),
		},
		Object: like,
	}
	AddContext(u)
	return u
}

// RenderAnnounce returns an Announce activity for a pure renote.
// targetURI は元ノートの URI (リモート / ローカル)。renoteID は renote 自身の ID。
func (r *Renderer) RenderAnnounce(renoter *model.User, renoteID string, targetURI string) *Announce {
	a := &Announce{
		Activity: Activity{
			Object: Object{
				ID:   r.urls.NoteURI(renoteID) + "/activity",
				Type: "Announce",
			},
			Actor: r.urls.UserURI(renoter.ID),
			To:    []string{Public},
			CC:    []string{r.urls.UserFollowers(renoter.ID)},
		},
		Object: targetURI,
	}
	AddContext(a)
	return a
}

// RenderUndoAnnounce wraps a previously emitted Announce in an Undo activity.
func (r *Renderer) RenderUndoAnnounce(renoter *model.User, announce *Announce) *Undo {
	u := &Undo{
		Activity: Activity{
			Object: Object{
				ID:   announce.ID + "/undo",
				Type: "Undo",
			},
			Actor: r.urls.UserURI(renoter.ID),
		},
		Object: announce,
	}
	AddContext(u)
	return u
}

// RenderDelete returns a Delete activity targeting the note URI. Object is a
// Tombstone so receivers can match it to a previously known note.
func (r *Renderer) RenderDelete(author *model.User, noteURI string) *Delete {
	d := &Delete{
		Activity: Activity{
			Object: Object{
				ID:   noteURI + "#Delete",
				Type: "Delete",
			},
			Actor: r.urls.UserURI(author.ID),
			To:    []string{Public},
		},
		Object: Tombstone{
			ID:   noteURI,
			Type: "Tombstone",
		},
	}
	AddContext(d)
	return d
}

// addressing computes to/cc lists for a note based on visibility.
func (r *Renderer) addressing(n *model.Note) (to []string, cc []string) {
	switch n.Visibility {
	case model.NoteVisibilityPublic:
		to = []string{Public}
		cc = []string{r.urls.UserFollowers(n.UserID)}
	case model.NoteVisibilityHome:
		to = []string{r.urls.UserFollowers(n.UserID)}
		cc = []string{Public}
	case model.NoteVisibilityFollowers:
		to = []string{r.urls.UserFollowers(n.UserID)}
	case model.NoteVisibilitySpecified:
		to = make([]string, 0, len(n.VisibleUserIDs))
		for _, uid := range n.VisibleUserIDs {
			to = append(to, r.urls.UserURI(uid))
		}
	}
	return to, cc
}

func stringValue(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func parseNoteTime(noteID string, idGen id.Generator) string {
	t, err := idGen.ParseTime(noteID)
	if err != nil {
		return time.Now().UTC().Format(time.RFC3339)
	}
	return t.UTC().Format(time.RFC3339)
}
