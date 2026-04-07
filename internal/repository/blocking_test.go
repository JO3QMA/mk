package repository

import (
	"context"
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cleanupBlocking(t *testing.T, id string) {
	t.Helper()
	testDB.Exec(`DELETE FROM "blocking" WHERE id = ?`, id)
}

func TestBlockingRepository_CRUD(t *testing.T) {
	repo := NewBlockingRepository(testDB)
	u1 := insertTestUser(t, "u_blk_1", "blk1")
	u2 := insertTestUser(t, "u_blk_2", "blk2")
	defer cleanupUser(t, u1.ID)
	defer cleanupUser(t, u2.ID)

	b := &model.Blocking{ID: "b1", BlockerID: u1.ID, BlockeeID: u2.ID}
	require.NoError(t, repo.Create(b))
	defer cleanupBlocking(t, b.ID)

	found, err := repo.FindByPair(u1.ID, u2.ID)
	require.NoError(t, err)
	assert.Equal(t, "b1", found.ID)

	exists, err := repo.Exists(u1.ID, u2.ID)
	require.NoError(t, err)
	assert.True(t, exists)

	exists, err = repo.Exists(u2.ID, u1.ID)
	require.NoError(t, err)
	assert.False(t, exists)

	rows, err := repo.ListByBlocker(u1.ID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, rows, 1)

	require.NoError(t, repo.Delete(b))
	_, err = repo.FindByPair(u1.ID, u2.ID)
	assert.Error(t, err)
}

func TestBlockingRepository_QueryErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := NewBlockingRepository(testDB.WithContext(ctx))

	err := repo.Create(&model.Blocking{ID: "x", BlockerID: "a", BlockeeID: "b"})
	assert.Error(t, err)
	_, err = repo.FindByPair("a", "b")
	assert.Error(t, err)
	_, err = repo.Exists("a", "b")
	assert.Error(t, err)
	_, err = repo.ListByBlocker("a", 10, 0)
	assert.Error(t, err)
}
