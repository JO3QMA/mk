package entity

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func newTestIDGen(t *testing.T) id.Generator {
	t.Helper()
	g, err := id.NewGenerator("aidx")
	require.NoError(t, err)
	return g
}

func TestPackNote_Basic(t *testing.T) {
	idGen := newTestIDGen(t)

	text := "Hello, world!"
	cw := "CW"
	noteID := idGen.Generate(time.Now())
	note := &model.Note{
		ID:         noteID,
		UserID:     "user1",
		Text:       &text,
		CW:         &cw,
		Visibility: model.NoteVisibilityPublic,
		Reactions:  datatypes.JSON([]byte("{}")),
		FileIDs:    pq.StringArray{"file1", "file2"},
		Tags:       pq.StringArray{"tag1"},
	}

	entity := PackNote(note, idGen)

	assert.Equal(t, noteID, entity.ID)
	assert.Equal(t, "user1", entity.UserID)
	assert.Equal(t, &text, entity.Text)
	assert.Equal(t, &cw, entity.CW)
	assert.Equal(t, "public", entity.Visibility)
	assert.Equal(t, []string{"file1", "file2"}, entity.FileIDs)
	assert.Equal(t, []string{"tag1"}, entity.Tags)
	assert.NotEmpty(t, entity.CreatedAt)
	assert.NotNil(t, entity.Emojis)
	assert.Empty(t, entity.Emojis)
	assert.NotNil(t, entity.Files)
	assert.Empty(t, entity.Files)
}

func TestPackNote_NilArrays(t *testing.T) {
	idGen := newTestIDGen(t)
	noteID := idGen.Generate(time.Now())

	note := &model.Note{
		ID:         noteID,
		UserID:     "user1",
		Visibility: model.NoteVisibilityHome,
		Reactions:  datatypes.JSON([]byte("{}")),
	}

	entity := PackNote(note, idGen)

	// FileIDsはnilでも空スライスに変換、Tagsはomitemptyでnil維持
	assert.NotNil(t, entity.FileIDs)
	assert.Empty(t, entity.FileIDs)
	assert.Nil(t, entity.Tags) // omitempty: nil → JSON省略
}

func TestPackNote_WithUser(t *testing.T) {
	idGen := newTestIDGen(t)
	noteID := idGen.Generate(time.Now())

	note := &model.Note{
		ID:         noteID,
		UserID:     "user1",
		Visibility: model.NoteVisibilityPublic,
		Reactions:  datatypes.JSON([]byte("{}")),
		User: &model.User{
			ID:                "user1",
			Username:          "testuser",
			AvatarDecorations: datatypes.JSON([]byte("[]")),
		},
	}

	entity := PackNote(note, idGen)

	assert.Equal(t, "user1", entity.User.ID)
	assert.Equal(t, "testuser", entity.User.Username)
}

func TestPackNote_ReactionCount(t *testing.T) {
	idGen := newTestIDGen(t)
	noteID := idGen.Generate(time.Now())

	note := &model.Note{
		ID:         noteID,
		UserID:     "user1",
		Visibility: model.NoteVisibilityPublic,
		Reactions:  datatypes.JSON([]byte(`{"👍":3,"❤️":2,"🎉":1}`)),
	}

	entity := PackNote(note, idGen)
	assert.Equal(t, 6, entity.ReactionCount)
}

func TestPackNote_ReactionCount_Empty(t *testing.T) {
	idGen := newTestIDGen(t)
	noteID := idGen.Generate(time.Now())

	note := &model.Note{
		ID:         noteID,
		UserID:     "user1",
		Visibility: model.NoteVisibilityPublic,
		Reactions:  datatypes.JSON([]byte("{}")),
	}

	entity := PackNote(note, idGen)
	assert.Equal(t, 0, entity.ReactionCount)
}

func TestPackNote_ReactionCount_NilReactions(t *testing.T) {
	idGen := newTestIDGen(t)
	noteID := idGen.Generate(time.Now())

	note := &model.Note{
		ID:         noteID,
		UserID:     "user1",
		Visibility: model.NoteVisibilityPublic,
	}

	entity := PackNote(note, idGen)
	assert.Equal(t, 0, entity.ReactionCount)
}

func TestPackNote_ReactionCount_InvalidJSON(t *testing.T) {
	idGen := newTestIDGen(t)
	noteID := idGen.Generate(time.Now())

	note := &model.Note{
		ID:         noteID,
		UserID:     "user1",
		Visibility: model.NoteVisibilityPublic,
		Reactions:  datatypes.JSON([]byte("invalid")),
	}

	entity := PackNote(note, idGen)
	assert.Equal(t, 0, entity.ReactionCount)
}

func TestPackNote_VisibleUserIDs_Mentions_HasPoll(t *testing.T) {
	idGen := newTestIDGen(t)
	noteID := idGen.Generate(time.Now())

	note := &model.Note{
		ID:             noteID,
		UserID:         "user1",
		Visibility:     model.NoteVisibilitySpecified,
		Reactions:      datatypes.JSON([]byte("{}")),
		VisibleUserIDs: pq.StringArray{"user2", "user3"},
		Mentions:       pq.StringArray{"user2"},
		HasPoll:        true,
	}

	entity := PackNote(note, idGen)

	assert.Equal(t, []string{"user2", "user3"}, entity.VisibleUserIDs)
	assert.Equal(t, []string{"user2"}, entity.Mentions)
	assert.True(t, entity.HasPoll)
}

func TestPackNote_NilVisibleUserIDs_Mentions(t *testing.T) {
	idGen := newTestIDGen(t)
	noteID := idGen.Generate(time.Now())

	note := &model.Note{
		ID:         noteID,
		UserID:     "user1",
		Visibility: model.NoteVisibilityPublic,
		Reactions:  datatypes.JSON([]byte("{}")),
	}

	entity := PackNote(note, idGen)

	// nil → empty slice
	assert.NotNil(t, entity.VisibleUserIDs)
	assert.Empty(t, entity.VisibleUserIDs)
	assert.NotNil(t, entity.Mentions)
	assert.Empty(t, entity.Mentions)
	assert.False(t, entity.HasPoll)
}

func TestNormalizeReactionKey(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"local legacy", ":smile:", ":smile@.:"},
		{"local canonical", ":smile@.:", ":smile@.:"},
		{"remote", ":smile@remote.example:", ":smile@remote.example:"},
		{"unicode emoji", "👍", "👍"},
		{"heart", "❤", "❤"},
		{"empty", "", ""},
		{"hyphen name", ":cat-smile:", ":cat-smile@.:"},
		{"plus name", ":thumbs+up:", ":thumbs+up@.:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, NormalizeReactionKey(tc.in))
		})
	}
}

func TestPackNote_ReactionsKeyNormalized(t *testing.T) {
	idGen := newTestIDGen(t)
	noteID := idGen.Generate(time.Now())

	// TS時代の `:lifelog:` とmk時代の `:lifelog@.:` が混在
	note := &model.Note{
		ID:         noteID,
		UserID:     "user1",
		Visibility: model.NoteVisibilityPublic,
		Reactions:  datatypes.JSON([]byte(`{":lifelog:":1,":lifelog@.:" :1,"👍":3}`)),
	}

	e := PackNote(note, idGen)

	var reactions map[string]float64
	require.NoError(t, json.Unmarshal(e.Reactions, &reactions))
	// `:lifelog:` と `:lifelog@.:` が統合される
	assert.Equal(t, float64(2), reactions[":lifelog@.:"])
	assert.Equal(t, float64(3), reactions["👍"])
	_, hasLegacy := reactions[":lifelog:"]
	assert.False(t, hasLegacy)
	assert.Equal(t, 5, e.ReactionCount)
}

func TestPackNote_WithRenoteAndReply(t *testing.T) {
	idGen := newTestIDGen(t)
	renoteID := idGen.Generate(time.Now())
	replyID := idGen.Generate(time.Now())
	noteID := idGen.Generate(time.Now())

	renoteText := "original"
	replyText := "parent reply"
	text := "quoting"

	note := &model.Note{
		ID:         noteID,
		UserID:     "user1",
		Text:       &text,
		RenoteID:   &renoteID,
		ReplyID:    &replyID,
		Visibility: model.NoteVisibilityPublic,
		Reactions:  datatypes.JSON([]byte("{}")),
		Renote: &model.Note{
			ID:         renoteID,
			UserID:     "user2",
			Text:       &renoteText,
			Visibility: model.NoteVisibilityPublic,
			Reactions:  datatypes.JSON([]byte("{}")),
			User: &model.User{
				ID:                "user2",
				Username:          "author2",
				AvatarDecorations: datatypes.JSON([]byte("[]")),
			},
		},
		Reply: &model.Note{
			ID:         replyID,
			UserID:     "user3",
			Text:       &replyText,
			Visibility: model.NoteVisibilityPublic,
			Reactions:  datatypes.JSON([]byte("{}")),
		},
	}

	e := PackNote(note, idGen)

	require.NotNil(t, e.Renote)
	assert.Equal(t, renoteID, e.Renote.ID)
	assert.Equal(t, &renoteText, e.Renote.Text)
	assert.Equal(t, "author2", e.Renote.User.Username)

	require.NotNil(t, e.Reply)
	assert.Equal(t, replyID, e.Reply.ID)
	assert.Equal(t, &replyText, e.Reply.Text)
}

func TestPackNotes_EmbeddedRenoteHasInstanceAndEmoji(t *testing.T) {
	idGen := newTestIDGen(t)
	renoteID := idGen.Generate(time.Now())
	noteID := idGen.Generate(time.Now())
	remoteHost := "remote.example"

	renote := &model.Note{
		ID:         renoteID,
		UserID:     "user2",
		Visibility: model.NoteVisibilityPublic,
		Reactions:  datatypes.JSON([]byte("{}")),
		UserHost:   &remoteHost,
		Emojis:     []string{"wave"},
		User: &model.User{
			ID:                "user2",
			Username:          "remoteuser",
			Host:              &remoteHost,
			AvatarDecorations: datatypes.JSON([]byte("[]")),
			Emojis:            []string{"wave"},
		},
	}
	note := &model.Note{
		ID:         noteID,
		UserID:     "user1",
		RenoteID:   &renoteID,
		Visibility: model.NoteVisibilityPublic,
		Reactions:  datatypes.JSON([]byte("{}")),
		User: &model.User{
			ID:                "user1",
			Username:          "localuser",
			AvatarDecorations: datatypes.JSON([]byte("[]")),
		},
		Renote: renote,
	}

	instLookup := &stubInstanceLookup{data: map[string]*model.Instance{
		remoteHost: {Host: remoteHost, Name: strPtr("Remote")},
	}}
	emojiLookup := &stubEmojiLookup{data: map[string][]*model.Emoji{
		remoteHost: {{Name: "wave", PublicURL: "https://remote.example/emoji/wave.png"}},
	}}

	out := PackNotes([]*model.Note{note}, idGen, instLookup, emojiLookup)
	require.Len(t, out, 1)
	require.NotNil(t, out[0].Renote)
	require.NotNil(t, out[0].Renote.User.Instance)
	assert.Equal(t, strPtr("Remote"), out[0].Renote.User.Instance.Name)
	assert.Equal(t, "https://remote.example/emoji/wave.png", out[0].Renote.Emojis["wave"])
	assert.Equal(t, "https://remote.example/emoji/wave.png", out[0].Renote.User.Emojis["wave"])
}

func TestPackNoteWithInstance_EmbeddedRenote(t *testing.T) {
	idGen := newTestIDGen(t)
	renoteID := idGen.Generate(time.Now())
	noteID := idGen.Generate(time.Now())
	remoteHost := "remote.example"

	note := &model.Note{
		ID:         noteID,
		UserID:     "user1",
		RenoteID:   &renoteID,
		Visibility: model.NoteVisibilityPublic,
		Reactions:  datatypes.JSON([]byte("{}")),
		Renote: &model.Note{
			ID:         renoteID,
			UserID:     "user2",
			Visibility: model.NoteVisibilityPublic,
			Reactions:  datatypes.JSON([]byte("{}")),
			UserHost:   &remoteHost,
			User: &model.User{
				ID:                "user2",
				Username:          "remoteuser",
				Host:              &remoteHost,
				AvatarDecorations: datatypes.JSON([]byte("[]")),
			},
		},
	}

	instLookup := &stubInstanceLookup{data: map[string]*model.Instance{
		remoteHost: {Host: remoteHost, Name: strPtr("Remote")},
	}}

	out := PackNoteWithInstance(note, idGen, instLookup, nil)
	require.NotNil(t, out.Renote)
	require.NotNil(t, out.Renote.User.Instance)
	assert.Equal(t, strPtr("Remote"), out.Renote.User.Instance.Name)
}

func TestFlattenNotesPlusRelations_NilSafe(t *testing.T) {
	out := flattenNotesPlusRelations([]*model.Note{nil, {ID: "a"}})
	require.Len(t, out, 1)
	assert.Equal(t, "a", out[0].ID)
}

func TestPackNote_CreatedAtParsing(t *testing.T) {
	idGen := newTestIDGen(t)
	noteID := idGen.Generate(time.Now())

	note := &model.Note{
		ID:         noteID,
		UserID:     "user1",
		Visibility: model.NoteVisibilityPublic,
		Reactions:  datatypes.JSON([]byte("{}")),
	}

	entity := PackNote(note, idGen)

	// createdAtはISO 8601形式
	assert.Contains(t, entity.CreatedAt, "T")
	assert.Contains(t, entity.CreatedAt, "Z")
}
