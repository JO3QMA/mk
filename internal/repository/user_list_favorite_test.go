package repository

import (
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedUserList は user_list_favorite の FK 制約を満たすためのヘルパー。
func seedUserList(t *testing.T, listID, ownerUserID string) {
	t.Helper()
	require.NoError(t, testDB.Exec(
		`INSERT INTO "user_list" (id, "userId", name, "isPublic") VALUES (?, ?, ?, true) ON CONFLICT (id) DO NOTHING`,
		listID, ownerUserID, "ul_"+listID,
	).Error)
	t.Cleanup(func() { testDB.Exec(`DELETE FROM "user_list" WHERE id = ?`, listID) })
}

func TestUserListFavoriteRepository_CRUD(t *testing.T) {
	repo := NewUserListFavoriteRepository(testDB)
	seedUser(t, "ulfav_u1")
	seedUserList(t, "ulfav_l1", "ulfav_u1")
	t.Cleanup(func() { testDB.Exec(`DELETE FROM "user_list_favorite" WHERE "userId" = ?`, "ulfav_u1") })

	fav := &model.UserListFavorite{ID: "ulfav_1", UserID: "ulfav_u1", UserListID: "ulfav_l1"}
	require.NoError(t, repo.Create(fav))

	ok, err := repo.Exists("ulfav_u1", "ulfav_l1")
	require.NoError(t, err)
	assert.True(t, ok)

	list, err := repo.ListByUser("ulfav_u1")
	require.NoError(t, err)
	assert.Len(t, list, 1)

	require.NoError(t, repo.Delete("ulfav_u1", "ulfav_l1"))
	ok, err = repo.Exists("ulfav_u1", "ulfav_l1")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestUserListFavoriteRepository_ListEmpty(t *testing.T) {
	repo := NewUserListFavoriteRepository(testDB)
	list, err := repo.ListByUser("ulfav_nonexistent")
	require.NoError(t, err)
	assert.Empty(t, list)
}
