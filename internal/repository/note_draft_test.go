package repository

import (
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoteDraftRepository_Full(t *testing.T) {
	repo := NewNoteDraftRepository(testDB)
	user := insertTestUser(t, "u_draft_1", "draftuser")
	defer cleanupUser(t, user.ID)

	text := "draft text"
	// Create
	draft := &model.NoteDraft{
		ID:         "draft_1",
		UserID:     user.ID,
		Text:       &text,
		Visibility: "public",
	}
	require.NoError(t, repo.Create(draft))
	defer testDB.Exec(`DELETE FROM "note_draft" WHERE id = ?`, draft.ID)

	// FindByIDAndUser
	found, err := repo.FindByIDAndUser("draft_1", user.ID)
	require.NoError(t, err)
	assert.Equal(t, "draft text", *found.Text)

	// FindByIDAndUser - not found
	_, err = repo.FindByIDAndUser("ghost", user.ID)
	assert.Error(t, err)

	// ListByUser
	list, err := repo.ListByUser(user.ID, 10)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	// ListByUser - default limit
	list2, err := repo.ListByUser(user.ID, 0)
	require.NoError(t, err)
	assert.Len(t, list2, 1)

	// Update
	newText := "updated text"
	found.Text = &newText
	require.NoError(t, repo.Update(found))
	updated, err := repo.FindByIDAndUser("draft_1", user.ID)
	require.NoError(t, err)
	assert.Equal(t, "updated text", *updated.Text)

	// CountByUser
	count, err := repo.CountByUser(user.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Delete
	require.NoError(t, repo.Delete("draft_1", user.ID))
	_, err = repo.FindByIDAndUser("draft_1", user.ID)
	assert.Error(t, err)

	// CountByUser after delete
	count, err = repo.CountByUser(user.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}
