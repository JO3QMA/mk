package repository

import (
	"context"
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertTestNote(t *testing.T, id, userID string) *model.Note {
	t.Helper()
	n := &model.Note{
		ID:         id,
		UserID:     userID,
		Visibility: model.NoteVisibilityPublic,
	}
	require.NoError(t, testDB.Create(n).Error)
	return n
}

func TestUserNotePiningRepository_Create_Find(t *testing.T) {
	repo := NewUserNotePiningRepository(testDB)
	user := insertTestUser(t, "u_pin_1", "pinuser1")
	defer cleanupUser(t, user.ID)
	note := insertTestNote(t, "n_pin_1", user.ID)
	defer testDB.Exec(`DELETE FROM "note" WHERE id = ?`, note.ID)

	p := &model.UserNotePining{
		ID:     "pin_1",
		UserID: user.ID,
		NoteID: note.ID,
	}
	require.NoError(t, repo.Create(p))
	defer testDB.Exec(`DELETE FROM "user_note_pining" WHERE id = ?`, p.ID)

	found, err := repo.FindByPair(user.ID, note.ID)
	require.NoError(t, err)
	assert.Equal(t, p.ID, found.ID)
}

func TestUserNotePiningRepository_FindByPair_NotFound(t *testing.T) {
	repo := NewUserNotePiningRepository(testDB)
	_, err := repo.FindByPair("nope", "nope")
	assert.Error(t, err)
}

func TestUserNotePiningRepository_Delete(t *testing.T) {
	repo := NewUserNotePiningRepository(testDB)
	user := insertTestUser(t, "u_pin_2", "pinuser2")
	defer cleanupUser(t, user.ID)
	note := insertTestNote(t, "n_pin_2", user.ID)
	defer testDB.Exec(`DELETE FROM "note" WHERE id = ?`, note.ID)

	p := &model.UserNotePining{ID: "pin_2", UserID: user.ID, NoteID: note.ID}
	require.NoError(t, repo.Create(p))

	require.NoError(t, repo.Delete(p))
	_, err := repo.FindByPair(user.ID, note.ID)
	assert.Error(t, err)
}

func TestUserNotePiningRepository_ListByUser_CountByUser(t *testing.T) {
	repo := NewUserNotePiningRepository(testDB)
	user := insertTestUser(t, "u_pin_3", "pinuser3")
	defer cleanupUser(t, user.ID)
	note1 := insertTestNote(t, "n_pin_3", user.ID)
	defer testDB.Exec(`DELETE FROM "note" WHERE id = ?`, note1.ID)
	note2 := insertTestNote(t, "n_pin_4", user.ID)
	defer testDB.Exec(`DELETE FROM "note" WHERE id = ?`, note2.ID)

	require.NoError(t, repo.Create(&model.UserNotePining{ID: "pin_3", UserID: user.ID, NoteID: note1.ID}))
	defer testDB.Exec(`DELETE FROM "user_note_pining" WHERE id = ?`, "pin_3")
	require.NoError(t, repo.Create(&model.UserNotePining{ID: "pin_4", UserID: user.ID, NoteID: note2.ID}))
	defer testDB.Exec(`DELETE FROM "user_note_pining" WHERE id = ?`, "pin_4")

	rows, err := repo.ListByUser(user.ID)
	require.NoError(t, err)
	assert.Len(t, rows, 2)

	count, err := repo.CountByUser(user.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestUserNotePiningRepository_QueryErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	db := testDB.WithContext(ctx)
	repo := NewUserNotePiningRepository(db)

	_, err := repo.ListByUser("a")
	assert.Error(t, err)

	_, err = repo.CountByUser("a")
	assert.Error(t, err)
}
