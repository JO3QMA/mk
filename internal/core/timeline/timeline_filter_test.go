package timeline

import (
	"testing"

	"github.com/lib/pq"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
)

func boolPtr(b bool) *bool    { return &b }
func strPtr(s string) *string { return &s }

// テスト用ノート生成ヘルパ
func makeNote(id string, opts ...func(*model.Note)) *model.Note {
	n := &model.Note{
		ID:         id,
		UserID:     "author",
		Visibility: model.NoteVisibilityPublic,
	}
	for _, o := range opts {
		o(n)
	}
	return n
}

func withFiles(n *model.Note)               { n.FileIDs = pq.StringArray{"file1"} }
func withRenote(n *model.Note)              { n.RenoteID = strPtr("rn1") }
func withText(n *model.Note)                { n.Text = strPtr("hello") }
func withReply(n *model.Note)               { n.ReplyID = strPtr("rp1"); n.ReplyUserID = strPtr("other") }
func withReplySelf(n *model.Note)           { n.ReplyID = strPtr("rp1"); n.ReplyUserID = strPtr("viewer") }
func withUser(uid string) func(*model.Note) { return func(n *model.Note) { n.UserID = uid } }
func withRenoteUser(uid string) func(*model.Note) {
	return func(n *model.Note) { n.RenoteUserID = strPtr(uid) }
}
func withRenoteUserHost(host string) func(*model.Note) {
	return func(n *model.Note) { n.RenoteUserHost = strPtr(host) }
}
func withLocalRenoteUser(n *model.Note) {
	n.RenoteUserHost = nil
}

func TestIsPureRenote(t *testing.T) {
	assert.True(t, isPureRenote(makeNote("1", withRenote)))
	assert.False(t, isPureRenote(makeNote("2")))
	assert.False(t, isPureRenote(makeNote("3", withRenote, withText)))
	assert.False(t, isPureRenote(makeNote("4", withRenote, withFiles)))
}

func TestApplyFilter_WithFiles(t *testing.T) {
	notes := []*model.Note{
		makeNote("1"),
		makeNote("2", withFiles),
	}
	out := ApplyFilter(notes, "", TimelineFilter{WithFiles: true})
	assert.Len(t, out, 1)
	assert.Equal(t, "2", out[0].ID)
}

func TestApplyFilter_WithRenotes_False(t *testing.T) {
	notes := []*model.Note{
		makeNote("1"),
		makeNote("2", withRenote),            // pure renote → 除外
		makeNote("3", withRenote, withText),  // quote renote → 残る
		makeNote("4", withRenote, withFiles), // quote renote (file) → 残る
	}
	out := ApplyFilter(notes, "", TimelineFilter{WithRenotes: boolPtr(false)})
	assert.Len(t, out, 3)
}

func TestApplyFilter_WithRenotes_Default(t *testing.T) {
	notes := []*model.Note{
		makeNote("1"),
		makeNote("2", withRenote),
	}
	// WithRenotes nil → true (全部残る)
	out := ApplyFilter(notes, "", TimelineFilter{})
	assert.Len(t, out, 2)
}

func TestApplyFilter_WithReplies_False(t *testing.T) {
	notes := []*model.Note{
		makeNote("1"),
		makeNote("2", withReply),     // 他人への返信 → 除外
		makeNote("3", withReplySelf), // 自分への返信 → 残る
	}
	out := ApplyFilter(notes, "viewer", TimelineFilter{WithReplies: boolPtr(false)})
	assert.Len(t, out, 2)
	assert.Equal(t, "1", out[0].ID)
	assert.Equal(t, "3", out[1].ID)
}

func TestApplyFilter_WithReplies_True(t *testing.T) {
	notes := []*model.Note{
		makeNote("1"),
		makeNote("2", withReply),
	}
	out := ApplyFilter(notes, "viewer", TimelineFilter{WithReplies: boolPtr(true)})
	assert.Len(t, out, 2)
}

func TestApplyFilter_WithReplies_NoViewer(t *testing.T) {
	notes := []*model.Note{
		makeNote("1"),
		makeNote("2", withReply),
	}
	// viewerなし + withReplies=false → 全返信除外
	out := ApplyFilter(notes, "", TimelineFilter{WithReplies: boolPtr(false)})
	assert.Len(t, out, 1)
}

func TestApplyFilter_IncludeMyRenotes_False(t *testing.T) {
	notes := []*model.Note{
		makeNote("1"),
		makeNote("2", withRenote, withUser("viewer")),           // viewerのpure renote → 除外
		makeNote("3", withRenote, withUser("other")),            // 他人のpure renote → 残る
		makeNote("4", withRenote, withText, withUser("viewer")), // viewerのquote → 残る
	}
	out := ApplyFilter(notes, "viewer", TimelineFilter{IncludeMyRenotes: boolPtr(false)})
	assert.Len(t, out, 3)
}

func TestApplyFilter_IncludeRenotedMyNotes_False(t *testing.T) {
	notes := []*model.Note{
		makeNote("1"),
		makeNote("2", withRenote, withRenoteUser("viewer")), // viewerのノートのpure renote → 除外
		makeNote("3", withRenote, withRenoteUser("other")),  // 他人のノートのpure renote → 残る
	}
	out := ApplyFilter(notes, "viewer", TimelineFilter{IncludeRenotedMyNotes: boolPtr(false)})
	assert.Len(t, out, 2)
}

func TestApplyFilter_IncludeLocalRenotes_False(t *testing.T) {
	notes := []*model.Note{
		makeNote("1"),
		makeNote("2", withRenote, withLocalRenoteUser),              // ローカルpure renote → 除外
		makeNote("3", withRenote, withRenoteUserHost("remote.tld")), // リモートpure renote → 残る
	}
	out := ApplyFilter(notes, "viewer", TimelineFilter{IncludeLocalRenotes: boolPtr(false)})
	assert.Len(t, out, 2)
}

func TestApplyFilter_CombinedFilters(t *testing.T) {
	notes := []*model.Note{
		makeNote("1", withFiles),                       // ファイル付き、通常ノート → 残る
		makeNote("2"),                                  // ファイルなし → 除外 (withFiles)
		makeNote("3", withRenote),                      // pure renote (ファイルなし) → 除外 (both)
		makeNote("4", withFiles, withRenote, withText), // ファイル付きquote → 残る
	}
	out := ApplyFilter(notes, "", TimelineFilter{
		WithFiles:   true,
		WithRenotes: boolPtr(false),
	})
	assert.Len(t, out, 2)
	assert.Equal(t, "1", out[0].ID)
	assert.Equal(t, "4", out[1].ID)
}

func TestBoolDefault(t *testing.T) {
	assert.True(t, boolDefault(nil, true))
	assert.False(t, boolDefault(nil, false))
	assert.True(t, boolDefault(boolPtr(true), false))
	assert.False(t, boolDefault(boolPtr(false), true))
}
