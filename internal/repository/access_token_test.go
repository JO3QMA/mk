package repository

import (
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccessTokenRepository_FindByHash(t *testing.T) {
	repo := NewAccessTokenRepository(testDB)
	user := insertTestUser(t, "u_atf_1", "tokenuser")
	defer cleanupUser(t, user.ID)

	token := &model.AccessToken{
		ID:     "at_atf_1",
		Token:  "rawtoken123",
		Hash:   "testhash123",
		UserID: user.ID,
	}
	require.NoError(t, testDB.Create(token).Error)
	defer testDB.Exec(`DELETE FROM "access_token" WHERE id = ?`, token.ID)

	found, err := repo.FindByHash("testhash123")
	require.NoError(t, err)
	assert.Equal(t, token.ID, found.ID)
	assert.NotNil(t, found.User)
	assert.Equal(t, user.ID, found.User.ID)
}

func TestAccessTokenRepository_FindByHash_NotFound(t *testing.T) {
	repo := NewAccessTokenRepository(testDB)

	_, err := repo.FindByHash("nonexistent_hash")
	assert.Error(t, err)
}
