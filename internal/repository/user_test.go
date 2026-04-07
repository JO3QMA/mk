package repository

import (
	"context"
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func insertTestUser(t *testing.T, id, username string) *model.User {
	t.Helper()
	token := "tok_" + id
	user := &model.User{
		ID:                id,
		Username:          username,
		UsernameLower:     username,
		Token:             &token,
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}
	require.NoError(t, testDB.Create(user).Error)
	return user
}

func cleanupUser(t *testing.T, id string) {
	t.Helper()
	testDB.Exec(`DELETE FROM "user_profile" WHERE "userId" = ?`, id)
	testDB.Exec(`DELETE FROM "user" WHERE id = ?`, id)
}

func TestUserRepository_FindByID(t *testing.T) {
	repo := NewUserRepository(testDB)
	user := insertTestUser(t, "u_fbi_1", "findbyid_user")
	defer cleanupUser(t, user.ID)

	found, err := repo.FindByID(user.ID)
	require.NoError(t, err)
	assert.Equal(t, user.ID, found.ID)
	assert.Equal(t, "findbyid_user", found.Username)
}

func TestUserRepository_FindByID_NotFound(t *testing.T) {
	repo := NewUserRepository(testDB)

	_, err := repo.FindByID("nonexistent_id")
	assert.Error(t, err)
}

func TestUserRepository_FindByToken(t *testing.T) {
	repo := NewUserRepository(testDB)
	user := insertTestUser(t, "u_fbt_1", "findbytoken_user")
	defer cleanupUser(t, user.ID)

	found, err := repo.FindByToken("tok_u_fbt_1")
	require.NoError(t, err)
	assert.Equal(t, user.ID, found.ID)
}

func TestUserRepository_FindByToken_NotFound(t *testing.T) {
	repo := NewUserRepository(testDB)

	_, err := repo.FindByToken("invalid_token")
	assert.Error(t, err)
}

func TestUserRepository_FindByUsernameLower_Local(t *testing.T) {
	repo := NewUserRepository(testDB)
	user := insertTestUser(t, "u_fun_1", "localuser")
	defer cleanupUser(t, user.ID)

	found, err := repo.FindByUsernameLower("localuser", nil)
	require.NoError(t, err)
	assert.Equal(t, user.ID, found.ID)
}

func TestUserRepository_FindByUsernameLower_Remote(t *testing.T) {
	repo := NewUserRepository(testDB)

	host := "remote.example.com"
	remoteUser := &model.User{
		ID:                "u_fun_2",
		Username:          "remoteuser",
		UsernameLower:     "remoteuser",
		Host:              &host,
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}
	require.NoError(t, testDB.Create(remoteUser).Error)
	defer cleanupUser(t, remoteUser.ID)

	found, err := repo.FindByUsernameLower("remoteuser", &host)
	require.NoError(t, err)
	assert.Equal(t, remoteUser.ID, found.ID)
	assert.Equal(t, &host, found.Host)
}

func TestUserRepository_FindProfileByUserID(t *testing.T) {
	repo := NewUserRepository(testDB)
	user := insertTestUser(t, "u_fpb_1", "profileuser")
	defer cleanupUser(t, user.ID)

	desc := "test description"
	profile := &model.UserProfile{
		UserID:      user.ID,
		Description: &desc,
		Fields:      datatypes.JSON([]byte("[]")),
	}
	require.NoError(t, testDB.Create(profile).Error)

	found, err := repo.FindProfileByUserID(user.ID)
	require.NoError(t, err)
	assert.Equal(t, &desc, found.Description)
}

func TestUserRepository_FindProfileByUserID_NotFound(t *testing.T) {
	repo := NewUserRepository(testDB)

	_, err := repo.FindProfileByUserID("nonexistent_user")
	assert.Error(t, err)
}

func TestUserRepository_FindByUsernameLower_NotFound(t *testing.T) {
	repo := NewUserRepository(testDB)

	_, err := repo.FindByUsernameLower("doesnotexist", nil)
	assert.Error(t, err)

	host := "nowhere.example.com"
	_, err = repo.FindByUsernameLower("doesnotexist", &host)
	assert.Error(t, err)
}

func TestUserRepository_SearchByUsername(t *testing.T) {
	repo := NewUserRepository(testDB)
	a := insertTestUser(t, "u_sb_1", "searchalpha")
	defer cleanupUser(t, a.ID)
	b := insertTestUser(t, "u_sb_2", "searchbeta")
	defer cleanupUser(t, b.ID)

	out, err := repo.SearchByUsername("search", 10, 0)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(out), 2)
}

func TestUserRepository_UpdateUser(t *testing.T) {
	repo := NewUserRepository(testDB)
	user := insertTestUser(t, "u_up_1", "updateuser1")
	defer cleanupUser(t, user.ID)

	require.NoError(t, repo.UpdateUser(user.ID, map[string]any{"isLocked": true}))
	found, _ := repo.FindByID(user.ID)
	assert.True(t, found.IsLocked)

	// 空フィールドはnoop
	require.NoError(t, repo.UpdateUser(user.ID, map[string]any{}))
}

func TestUserRepository_SearchByUsername_QueryError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	db := testDB.WithContext(ctx)
	repo := NewUserRepository(db)

	_, err := repo.SearchByUsername("anything", 10, 0)
	assert.Error(t, err)
}

func TestUserRepository_UpdateProfile(t *testing.T) {
	repo := NewUserRepository(testDB)
	user := insertTestUser(t, "u_up_2", "updateuser2")
	defer cleanupUser(t, user.ID)

	desc := "initial"
	profile := &model.UserProfile{
		UserID:      user.ID,
		Description: &desc,
		Fields:      datatypes.JSON([]byte("[]")),
	}
	require.NoError(t, testDB.Create(profile).Error)

	newDesc := "updated"
	require.NoError(t, repo.UpdateProfile(user.ID, map[string]any{"description": newDesc}))
	found, _ := repo.FindProfileByUserID(user.ID)
	assert.Equal(t, "updated", *found.Description)

	// 空フィールドはnoop
	require.NoError(t, repo.UpdateProfile(user.ID, map[string]any{}))
}
