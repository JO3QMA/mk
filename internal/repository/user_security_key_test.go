package repository

import (
	"testing"

	"github.com/lib/pq"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// cleanupSecurityKey wipes a row by primary key. Some tests insert keys
// directly to set up specific scenarios; this keeps DB state isolated.
func cleanupSecurityKey(t *testing.T, id string) {
	t.Helper()
	testDB.Exec(`DELETE FROM "user_security_key" WHERE id = ?`, id)
}

func TestUserSecurityKeyRepository_CRUD(t *testing.T) {
	repo := NewUserSecurityKeyRepository(testDB)
	user := insertTestUser(t, "u_sk_1", "sk_user")
	defer cleanupUser(t, user.ID)

	key := &model.UserSecurityKey{
		ID:         "key-1",
		UserID:     user.ID,
		Name:       "Yubikey",
		PublicKey:  "pk-1",
		Counter:    0,
		Transports: pq.StringArray{"usb"},
	}
	require.NoError(t, repo.Create(key))
	defer cleanupSecurityKey(t, key.ID)

	got, err := repo.FindByID(key.ID)
	require.NoError(t, err)
	assert.Equal(t, "Yubikey", got.Name)
	assert.Equal(t, "pk-1", got.PublicKey)
	assert.Equal(t, []string{"usb"}, []string(got.Transports))
	assert.False(t, got.LastUsed.IsZero(), "Create should populate LastUsed when zero")

	// CountByUser
	n, err := repo.CountByUser(user.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	// ListByUser ordering (by lastUsed DESC)
	keys, err := repo.ListByUser(user.ID)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.Equal(t, "key-1", keys[0].ID)

	// UpdateName ok
	require.NoError(t, repo.UpdateName(key.ID, user.ID, "Renamed"))
	got, _ = repo.FindByID(key.ID)
	assert.Equal(t, "Renamed", got.Name)

	// UpdateCounter advances counter + lastUsed
	prevLastUsed := got.LastUsed
	require.NoError(t, repo.UpdateCounter(key.ID, 5))
	got, _ = repo.FindByID(key.ID)
	assert.Equal(t, int64(5), got.Counter)
	assert.True(t, got.LastUsed.After(prevLastUsed) || got.LastUsed.Equal(prevLastUsed))

	// Delete
	require.NoError(t, repo.Delete(key.ID, user.ID))
	_, err = repo.FindByID(key.ID)
	assert.Error(t, err)
}

func TestUserSecurityKeyRepository_UpdateName_NotOwned(t *testing.T) {
	repo := NewUserSecurityKeyRepository(testDB)
	owner := insertTestUser(t, "u_sk_owner", "sk_owner")
	defer cleanupUser(t, owner.ID)
	other := insertTestUser(t, "u_sk_other", "sk_other")
	defer cleanupUser(t, other.ID)

	key := &model.UserSecurityKey{ID: "key-owned", UserID: owner.ID, Name: "k", PublicKey: "pk"}
	require.NoError(t, repo.Create(key))
	defer cleanupSecurityKey(t, key.ID)

	// 別のユーザーが rename しようとしても弾かれる
	err := repo.UpdateName(key.ID, other.ID, "stolen")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	// 元の name は変わっていない
	got, _ := repo.FindByID(key.ID)
	assert.Equal(t, "k", got.Name)
}

func TestUserSecurityKeyRepository_Delete_NotOwned(t *testing.T) {
	repo := NewUserSecurityKeyRepository(testDB)
	owner := insertTestUser(t, "u_skd_own", "skd_own")
	defer cleanupUser(t, owner.ID)
	other := insertTestUser(t, "u_skd_oth", "skd_oth")
	defer cleanupUser(t, other.ID)

	key := &model.UserSecurityKey{ID: "key-del", UserID: owner.ID, Name: "k", PublicKey: "pk"}
	require.NoError(t, repo.Create(key))
	defer cleanupSecurityKey(t, key.ID)

	err := repo.Delete(key.ID, other.ID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	// 行は残っている
	_, err = repo.FindByID(key.ID)
	assert.NoError(t, err)
}

func TestUserSecurityKeyRepository_FindByID_NotFound(t *testing.T) {
	repo := NewUserSecurityKeyRepository(testDB)
	_, err := repo.FindByID("ghost")
	assert.Error(t, err)
}

func TestUserSecurityKeyRepository_ListByUser_Empty(t *testing.T) {
	repo := NewUserSecurityKeyRepository(testDB)
	keys, err := repo.ListByUser("ghost-user-no-keys")
	require.NoError(t, err)
	assert.Empty(t, keys)
}

func TestUserSecurityKeyRepository_CountByUser_Multiple(t *testing.T) {
	repo := NewUserSecurityKeyRepository(testDB)
	user := insertTestUser(t, "u_sk_count", "sk_count")
	defer cleanupUser(t, user.ID)

	for i := 0; i < 3; i++ {
		key := &model.UserSecurityKey{
			ID:        "key-multi-" + string(rune('a'+i)),
			UserID:    user.ID,
			Name:      "k",
			PublicKey: "pk",
		}
		require.NoError(t, repo.Create(key))
		defer cleanupSecurityKey(t, key.ID)
	}

	n, err := repo.CountByUser(user.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(3), n)
}
