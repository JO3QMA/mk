package repository

import (
	"testing"

	"github.com/lib/pq"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func TestPollRepository_Create(t *testing.T) {
	repo := NewPollRepository(testDB)
	user := insertTestUser(t, "u_pc_1", "polluser")
	defer cleanupUser(t, user.ID)

	note := &model.Note{
		ID:         "n_pc_1",
		UserID:     user.ID,
		Visibility: model.NoteVisibilityPublic,
		HasPoll:    true,
		Reactions:  datatypes.JSON([]byte("{}")),
	}
	require.NoError(t, testDB.Create(note).Error)
	defer cleanupNote(t, note.ID)

	poll := &model.Poll{
		NoteID:         note.ID,
		Multiple:       false,
		Choices:        pq.StringArray{"A", "B", "C"},
		Votes:          pq.Int64Array{0, 0, 0},
		NoteVisibility: model.NoteVisibilityPublic,
		UserID:         user.ID,
	}
	require.NoError(t, repo.Create(poll))

	// 作成されたことを確認
	var found model.Poll
	err := testDB.First(&found, "\"noteId\" = ?", note.ID).Error
	require.NoError(t, err)
	assert.Equal(t, note.ID, found.NoteID)
	assert.Len(t, found.Choices, 3)
	assert.False(t, found.Multiple)
}
