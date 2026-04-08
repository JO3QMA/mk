package repository

import (
	"context"
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelFollowingRepository_LifeCycle(t *testing.T) {
	chRepo := NewChannelRepository(testDB)
	repo := NewChannelFollowingRepository(testDB)
	user := insertTestUser(t, "u_cf_1", "cfuser1")
	defer cleanupUser(t, user.ID)
	uid := user.ID
	c := newTestChannel("ch_cf_1", "follow target", &uid)
	require.NoError(t, chRepo.Create(c))
	defer cleanupChannel(t, c.ID)

	f := &model.ChannelFollowing{ID: "cf_pair_1", FollowerID: user.ID, FolloweeID: c.ID}
	require.NoError(t, repo.Create(f))

	got, err := repo.FindByPair(user.ID, c.ID)
	require.NoError(t, err)
	assert.Equal(t, f.ID, got.ID)

	exists, err := repo.Exists(user.ID, c.ID)
	require.NoError(t, err)
	assert.True(t, exists)

	rows, err := repo.ListFollowed(user.ID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, rows, 1)

	require.NoError(t, repo.Delete(f))
	exists, err = repo.Exists(user.ID, c.ID)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestChannelFollowingRepository_FindByPair_NotFound(t *testing.T) {
	repo := NewChannelFollowingRepository(testDB)
	_, err := repo.FindByPair("nope", "nope")
	assert.Error(t, err)
}

func TestChannelFollowingRepository_ListFollowed_LimitClamp(t *testing.T) {
	repo := NewChannelFollowingRepository(testDB)
	rows, err := repo.ListFollowed("nobody", 9999, 0)
	require.NoError(t, err)
	assert.Empty(t, rows)
	rows, err = repo.ListFollowed("nobody", -1, 0)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestChannelFollowingRepository_QueryErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	db := testDB.WithContext(ctx)
	repo := NewChannelFollowingRepository(db)
	_, err := repo.Exists("a", "b")
	assert.Error(t, err)
	_, err = repo.ListFollowed("a", 10, 0)
	assert.Error(t, err)
}
