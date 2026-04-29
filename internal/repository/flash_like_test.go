package repository

import (
	"context"
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlashLikeRepository_LifeCycle(t *testing.T) {
	flashRepo := NewFlashRepository(testDB)
	repo := NewFlashLikeRepository(testDB)
	user := insertTestUser(t, "u_flr_1", "fluser1")
	defer cleanupUser(t, user.ID)

	f := newTestFlash("fl_fl_1", user.ID, "alpha")
	require.NoError(t, flashRepo.Create(f))
	defer cleanupFlash(t, f.ID)

	fl := &model.FlashLike{ID: "fll_pair_1", UserID: user.ID, FlashID: f.ID}
	require.NoError(t, repo.Create(fl))
	defer testDB.Exec(`DELETE FROM "flash_like" WHERE id = ?`, fl.ID)

	got, err := repo.FindByPair(user.ID, f.ID)
	require.NoError(t, err)
	assert.Equal(t, fl.ID, got.ID)

	exists, err := repo.Exists(user.ID, f.ID)
	require.NoError(t, err)
	assert.True(t, exists)

	rows, err := repo.ListByUser(user.ID, "", "", 10, 0)
	require.NoError(t, err)
	assert.Len(t, rows, 1)

	require.NoError(t, repo.Delete(fl))
	_, err = repo.FindByPair(user.ID, f.ID)
	assert.Error(t, err)
}

func TestFlashLikeRepository_FindByPair_NotFound(t *testing.T) {
	repo := NewFlashLikeRepository(testDB)
	_, err := repo.FindByPair("nope", "nope")
	assert.Error(t, err)
}

func TestFlashLikeRepository_Exists_False(t *testing.T) {
	repo := NewFlashLikeRepository(testDB)
	exists, err := repo.Exists("nope", "nope")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestFlashLikeRepository_Exists_QueryError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	db := testDB.WithContext(ctx)
	repo := NewFlashLikeRepository(db)
	_, err := repo.Exists("any", "any")
	assert.Error(t, err)
}

func TestFlashLikeRepository_ListByUser_LimitClamp(t *testing.T) {
	repo := NewFlashLikeRepository(testDB)
	_, err := repo.ListByUser("nobody", "", "", 9999, 0)
	require.NoError(t, err)
	_, err = repo.ListByUser("nobody", "", "", -1, 0)
	require.NoError(t, err)
}

func TestFlashLikeRepository_ListByUser_QueryError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	db := testDB.WithContext(ctx)
	repo := NewFlashLikeRepository(db)
	_, err := repo.ListByUser("nobody", "", "", 10, 0)
	assert.Error(t, err)
}
