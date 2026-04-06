package repository

import (
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func cleanupNote(t *testing.T, id string) {
	t.Helper()
	testDB.Exec(`DELETE FROM "poll" WHERE "noteId" = ?`, id)
	testDB.Exec(`DELETE FROM "note" WHERE id = ?`, id)
}

func TestNoteRepository_CreateAndFindByID(t *testing.T) {
	repo := NewNoteRepository(testDB)
	user := insertTestUser(t, "u_ncf_1", "noteuser1")
	defer cleanupUser(t, user.ID)

	text := "Hello, world!"
	note := &model.Note{
		ID:         "n_ncf_1",
		UserID:     user.ID,
		Text:       &text,
		Visibility: model.NoteVisibilityPublic,
		Reactions:  datatypes.JSON([]byte("{}")),
	}
	require.NoError(t, repo.Create(note))
	defer cleanupNote(t, note.ID)

	found, err := repo.FindByID(note.ID)
	require.NoError(t, err)
	assert.Equal(t, note.ID, found.ID)
	assert.Equal(t, &text, found.Text)
}

func TestNoteRepository_FindByIDWithUser(t *testing.T) {
	repo := NewNoteRepository(testDB)
	user := insertTestUser(t, "u_nfu_1", "noteuser2")
	defer cleanupUser(t, user.ID)

	note := &model.Note{
		ID:         "n_nfu_1",
		UserID:     user.ID,
		Visibility: model.NoteVisibilityHome,
		Reactions:  datatypes.JSON([]byte("{}")),
	}
	require.NoError(t, repo.Create(note))
	defer cleanupNote(t, note.ID)

	found, err := repo.FindByIDWithUser(note.ID)
	require.NoError(t, err)
	assert.NotNil(t, found.User)
	assert.Equal(t, user.ID, found.User.ID)
	assert.Equal(t, "noteuser2", found.User.Username)
}

func TestNoteRepository_Delete(t *testing.T) {
	repo := NewNoteRepository(testDB)
	user := insertTestUser(t, "u_nd_1", "noteuser3")
	defer cleanupUser(t, user.ID)

	note := &model.Note{
		ID:         "n_nd_1",
		UserID:     user.ID,
		Visibility: model.NoteVisibilityPublic,
		Reactions:  datatypes.JSON([]byte("{}")),
	}
	require.NoError(t, repo.Create(note))

	require.NoError(t, repo.Delete(note))

	_, err := repo.FindByID(note.ID)
	assert.Error(t, err)
}

func TestNoteRepository_Update(t *testing.T) {
	repo := NewNoteRepository(testDB)
	user := insertTestUser(t, "u_nu_1", "noteuser4")
	defer cleanupUser(t, user.ID)

	note := &model.Note{
		ID:         "n_nu_1",
		UserID:     user.ID,
		Visibility: model.NoteVisibilityPublic,
		Reactions:  datatypes.JSON([]byte("{}")),
	}
	require.NoError(t, repo.Create(note))
	defer cleanupNote(t, note.ID)

	require.NoError(t, repo.Update(note, "hasPoll", true))

	found, err := repo.FindByID(note.ID)
	require.NoError(t, err)
	assert.True(t, found.HasPoll)
}

func TestNoteRepository_FindByID_NotFound(t *testing.T) {
	repo := NewNoteRepository(testDB)

	_, err := repo.FindByID("nonexistent_note")
	assert.Error(t, err)
}

func TestNoteRepository_FindByIDWithUser_NotFound(t *testing.T) {
	repo := NewNoteRepository(testDB)

	_, err := repo.FindByIDWithUser("nonexistent_note")
	assert.Error(t, err)
}
