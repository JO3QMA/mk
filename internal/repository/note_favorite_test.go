package repository

import (
	"context"
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoteFavoriteRepository_CRUD(t *testing.T) {
	repo := NewNoteFavoriteRepository(testDB)
	user := insertTestUser(t, "fav_u1", "favuser")
	defer cleanupUser(t, user.ID)

	note := &model.Note{ID: "fav_n1", UserID: user.ID, Visibility: "public"}
	require.NoError(t, testDB.Create(note).Error)
	defer testDB.Exec(`DELETE FROM "note" WHERE id = ?`, note.ID)

	f := &model.NoteFavorite{ID: "fav_1", UserID: user.ID, NoteID: note.ID}
	require.NoError(t, repo.Create(f))
	defer testDB.Exec(`DELETE FROM "note_favorite" WHERE id = ?`, f.ID)

	exists, err := repo.Exists(user.ID, note.ID)
	require.NoError(t, err)
	assert.True(t, exists)

	favs, err := repo.ListByUser(user.ID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, favs, 1)

	// default limit
	favs, err = repo.ListByUser(user.ID, 0, 0)
	require.NoError(t, err)
	assert.NotEmpty(t, favs)

	// limit cap
	favs, err = repo.ListByUser(user.ID, 999, 0)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(favs), 100)

	// offset
	favs, err = repo.ListByUser(user.ID, 10, 99)
	require.NoError(t, err)
	assert.Empty(t, favs)

	require.NoError(t, repo.Delete(user.ID, note.ID))
	exists, _ = repo.Exists(user.ID, note.ID)
	assert.False(t, exists)
}

func TestNoteFavoriteRepository_Exists_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := NewNoteFavoriteRepository(testDB.WithContext(ctx))
	_, err := repo.Exists("x", "y")
	assert.Error(t, err)
}

func TestNoteFavoriteRepository_ListByUser_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := NewNoteFavoriteRepository(testDB.WithContext(ctx))
	_, err := repo.ListByUser("x", 10, 0)
	assert.Error(t, err)
}
