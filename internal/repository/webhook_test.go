package repository

import (
	"testing"

	"github.com/lib/pq"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookRepository_Full(t *testing.T) {
	repo := NewWebhookRepository(testDB)
	user := insertTestUser(t, "u_wh_1", "webhookuser")
	defer cleanupUser(t, user.ID)

	// Create
	w := &model.Webhook{
		ID:     "wh_1",
		UserID: user.ID,
		Name:   "Test Hook",
		On:     pq.StringArray{"note", "follow"},
		URL:    "https://example.com/hook",
		Secret: "secret123",
		Active: true,
	}
	require.NoError(t, repo.Create(w))
	defer testDB.Exec(`DELETE FROM "webhook" WHERE id = ?`, w.ID)

	// FindByID
	found, err := repo.FindByID("wh_1")
	require.NoError(t, err)
	assert.Equal(t, "Test Hook", found.Name)

	// FindByID - not found
	_, err = repo.FindByID("ghost")
	assert.Error(t, err)

	// FindByIDAndUserID
	found2, err := repo.FindByIDAndUserID("wh_1", user.ID)
	require.NoError(t, err)
	assert.Equal(t, "Test Hook", found2.Name)

	// FindByIDAndUserID - wrong user
	_, err = repo.FindByIDAndUserID("wh_1", "wrong")
	assert.Error(t, err)

	// ListByUserID
	list, err := repo.ListByUserID(user.ID)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	// Update
	found.Name = "Updated Hook"
	require.NoError(t, repo.Update(found))
	updated, err := repo.FindByID("wh_1")
	require.NoError(t, err)
	assert.Equal(t, "Updated Hook", updated.Name)

	// Delete
	require.NoError(t, repo.Delete("wh_1", user.ID))
	_, err = repo.FindByID("wh_1")
	assert.Error(t, err)
}
