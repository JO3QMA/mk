package entity

import (
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

	// nil配列は空スライスに変換される
	assert.NotNil(t, entity.FileIDs)
	assert.Empty(t, entity.FileIDs)
	assert.NotNil(t, entity.Tags)
	assert.Empty(t, entity.Tags)
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
