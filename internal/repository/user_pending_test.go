package repository

import (
	"context"
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserPendingRepository_CRUD(t *testing.T) {
	repo := NewUserPendingRepository(testDB)

	p := &model.UserPending{
		ID:       "up_1",
		Code:     "test_code_abc",
		Username: "pendinguser",
		Email:    "pending@example.com",
		Password: "$2a$10$fakehash",
	}
	require.NoError(t, repo.Create(p))
	defer testDB.Exec(`DELETE FROM "user_pending" WHERE id = ?`, p.ID)

	// FindByCode
	found, err := repo.FindByCode("test_code_abc")
	require.NoError(t, err)
	assert.Equal(t, "up_1", found.ID)
	assert.Equal(t, "pendinguser", found.Username)
	assert.Equal(t, "pending@example.com", found.Email)

	// FindByCode: not found
	_, err = repo.FindByCode("nonexistent")
	assert.Error(t, err)

	// Delete
	require.NoError(t, repo.Delete("up_1"))
	_, err = repo.FindByCode("test_code_abc")
	assert.Error(t, err)
}

func TestUserPendingRepository_Errors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := NewUserPendingRepository(testDB.WithContext(ctx))

	_, err := repo.FindByCode("x")
	assert.Error(t, err)
}
