package repository

import (
	"context"
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertFollowRequest(t *testing.T, id, followerID, followeeID string) *model.FollowRequest {
	t.Helper()
	r := &model.FollowRequest{
		ID:         id,
		FollowerID: followerID,
		FolloweeID: followeeID,
	}
	require.NoError(t, testDB.Create(r).Error)
	return r
}

func TestFollowRequestRepository_Create_FindByPair(t *testing.T) {
	repo := NewFollowRequestRepository(testDB)
	follower := insertTestUser(t, "u_fr_1", "frfollower1")
	defer cleanupUser(t, follower.ID)
	followee := insertTestUser(t, "u_fr_2", "frfollowee1")
	defer cleanupUser(t, followee.ID)

	r := &model.FollowRequest{
		ID:         "fr_1",
		FollowerID: follower.ID,
		FolloweeID: followee.ID,
	}
	require.NoError(t, repo.Create(r))
	defer testDB.Exec(`DELETE FROM "follow_request" WHERE id = ?`, r.ID)

	found, err := repo.FindByPair(follower.ID, followee.ID)
	require.NoError(t, err)
	assert.Equal(t, r.ID, found.ID)
}

func TestFollowRequestRepository_FindByPair_NotFound(t *testing.T) {
	repo := NewFollowRequestRepository(testDB)
	_, err := repo.FindByPair("ghost1", "ghost2")
	assert.Error(t, err)
}

func TestFollowRequestRepository_Exists(t *testing.T) {
	repo := NewFollowRequestRepository(testDB)
	follower := insertTestUser(t, "u_fr_3", "frfollower3")
	defer cleanupUser(t, follower.ID)
	followee := insertTestUser(t, "u_fr_4", "frfollowee4")
	defer cleanupUser(t, followee.ID)

	exists, err := repo.Exists(follower.ID, followee.ID)
	require.NoError(t, err)
	assert.False(t, exists)

	insertFollowRequest(t, "fr_2", follower.ID, followee.ID)
	defer testDB.Exec(`DELETE FROM "follow_request" WHERE id = ?`, "fr_2")

	exists, err = repo.Exists(follower.ID, followee.ID)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestFollowRequestRepository_Delete(t *testing.T) {
	repo := NewFollowRequestRepository(testDB)
	follower := insertTestUser(t, "u_fr_5", "frfollower5")
	defer cleanupUser(t, follower.ID)
	followee := insertTestUser(t, "u_fr_6", "frfollowee6")
	defer cleanupUser(t, followee.ID)

	r := insertFollowRequest(t, "fr_3", follower.ID, followee.ID)
	require.NoError(t, repo.Delete(r))

	exists, err := repo.Exists(follower.ID, followee.ID)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestFollowRequestRepository_QueryErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	db := testDB.WithContext(ctx)
	repo := NewFollowRequestRepository(db)

	_, err := repo.Exists("a", "b")
	assert.Error(t, err)

	_, err = repo.ListReceived("a", 10, "", "")
	assert.Error(t, err)

	_, err = repo.ListSent("a", 10, "", "")
	assert.Error(t, err)
}

func TestFollowRequestRepository_ListReceived_ListSent(t *testing.T) {
	repo := NewFollowRequestRepository(testDB)
	a := insertTestUser(t, "u_fr_7", "fruser7")
	defer cleanupUser(t, a.ID)
	b := insertTestUser(t, "u_fr_8", "fruser8")
	defer cleanupUser(t, b.ID)
	c := insertTestUser(t, "u_fr_9", "fruser9")
	defer cleanupUser(t, c.ID)

	insertFollowRequest(t, "fr_4", a.ID, b.ID)
	defer testDB.Exec(`DELETE FROM "follow_request" WHERE id = ?`, "fr_4")
	insertFollowRequest(t, "fr_5", c.ID, b.ID)
	defer testDB.Exec(`DELETE FROM "follow_request" WHERE id = ?`, "fr_5")
	insertFollowRequest(t, "fr_6", a.ID, c.ID)
	defer testDB.Exec(`DELETE FROM "follow_request" WHERE id = ?`, "fr_6")

	received, err := repo.ListReceived(b.ID, 10, "", "")
	require.NoError(t, err)
	assert.Len(t, received, 2)

	sent, err := repo.ListSent(a.ID, 10, "", "")
	require.NoError(t, err)
	assert.Len(t, sent, 2)
}
