package repository

import (
	"context"
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPageLikeRepository_LifeCycle(t *testing.T) {
	pageRepo := NewPageRepository(testDB)
	repo := NewPageLikeRepository(testDB)
	user := insertTestUser(t, "u_plr_1", "pluser1")
	defer cleanupUser(t, user.ID)

	p := newTestPage("pg_pl_1", user.ID, "alpha")
	require.NoError(t, pageRepo.Create(p))
	defer cleanupPage(t, p.ID)

	pl := &model.PageLike{ID: "pl_pair_1", UserID: user.ID, PageID: p.ID}
	require.NoError(t, repo.Create(pl))
	defer testDB.Exec(`DELETE FROM "page_like" WHERE id = ?`, pl.ID)

	got, err := repo.FindByPair(user.ID, p.ID)
	require.NoError(t, err)
	assert.Equal(t, pl.ID, got.ID)

	exists, err := repo.Exists(user.ID, p.ID)
	require.NoError(t, err)
	assert.True(t, exists)

	require.NoError(t, repo.Delete(pl))
	_, err = repo.FindByPair(user.ID, p.ID)
	assert.Error(t, err)
}

func TestPageLikeRepository_FindByPair_NotFound(t *testing.T) {
	repo := NewPageLikeRepository(testDB)
	_, err := repo.FindByPair("nope", "nope")
	assert.Error(t, err)
}

func TestPageLikeRepository_Exists_False(t *testing.T) {
	repo := NewPageLikeRepository(testDB)
	exists, err := repo.Exists("nope", "nope")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestPageLikeRepository_Exists_QueryError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	db := testDB.WithContext(ctx)
	repo := NewPageLikeRepository(db)
	_, err := repo.Exists("any", "any")
	assert.Error(t, err)
}
