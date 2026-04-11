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

// insertRemoteTestUser inserts a user record marked as belonging to the
// given remote host. SearchByFilter / federation 系のテスト用ヘルパ。
func insertRemoteTestUser(t *testing.T, id, username, host string) *model.User {
	t.Helper()
	user := &model.User{
		ID:                id,
		Username:          username,
		UsernameLower:     username,
		Host:              &host,
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}
	require.NoError(t, testDB.Create(user).Error)
	return user
}

func TestUserRepository_CreateAndFindByURI(t *testing.T) {
	repo := NewUserRepository(testDB)
	uri := "https://remote.example/users/remote1"
	host := "remote.example"
	u := &model.User{
		ID:                "remote1",
		Username:          "remote1",
		UsernameLower:     "remote1",
		URI:               &uri,
		Host:              &host,
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}
	require.NoError(t, repo.Create(u))
	defer cleanupUser(t, u.ID)

	got, err := repo.FindByURI(uri)
	require.NoError(t, err)
	assert.Equal(t, "remote1", got.ID)
}

func TestUserRepository_FindByURI_NotFound(t *testing.T) {
	repo := NewUserRepository(testDB)
	_, err := repo.FindByURI("https://nope.example/x")
	assert.Error(t, err)
}

func TestUserRepository_Create_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := NewUserRepository(testDB.WithContext(ctx))
	err := repo.Create(&model.User{ID: "x", Username: "x", UsernameLower: "x"})
	assert.Error(t, err)
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

func TestUserRepository_CreateProfile(t *testing.T) {
	repo := NewUserRepository(testDB)
	user := insertTestUser(t, "cp_u1", "cpuser")
	defer cleanupUser(t, user.ID)

	pass := "$2a$10$test"
	profile := &model.UserProfile{
		UserID:             user.ID,
		Password:           &pass,
		AutoAcceptFollowed: true,
	}
	require.NoError(t, repo.CreateProfile(profile))

	found, err := repo.FindProfileByUserID(user.ID)
	require.NoError(t, err)
	assert.Equal(t, &pass, found.Password)
}

func TestUserRepository_CreateProfile_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := NewUserRepository(testDB.WithContext(ctx))
	_, err := repo.ListUsers(model.UserListFilter{Origin: "combined", State: "all", Sort: "invalid"})
	assert.Error(t, err)
}

func TestUserRepository_ListUsers_Default(t *testing.T) {
	repo := NewUserRepository(testDB)
	u1 := insertTestUser(t, "lu_u1", "listuser1")
	u2 := insertTestUser(t, "lu_u2", "listuser2")
	defer cleanupUser(t, u1.ID)
	defer cleanupUser(t, u2.ID)

	users, err := repo.ListUsers(model.UserListFilter{Limit: 100})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(users), 2)
}

func TestUserRepository_ListUsers_LocalOnly(t *testing.T) {
	repo := NewUserRepository(testDB)
	u := insertTestUser(t, "lu_loc", "localonly")
	defer cleanupUser(t, u.ID)

	users, err := repo.ListUsers(model.UserListFilter{Origin: "local", Limit: 100})
	require.NoError(t, err)
	for _, user := range users {
		assert.Nil(t, user.Host)
	}
}

func TestUserRepository_ListUsers_Suspended(t *testing.T) {
	repo := NewUserRepository(testDB)
	u := insertTestUser(t, "lu_sus", "suspended")
	defer cleanupUser(t, u.ID)
	require.NoError(t, repo.UpdateUser(u.ID, map[string]any{"isSuspended": true}))

	users, err := repo.ListUsers(model.UserListFilter{State: "suspended", Limit: 100})
	require.NoError(t, err)
	for _, user := range users {
		assert.True(t, user.IsSuspended)
	}
}

func TestUserRepository_ListUsers_SortAndPagination(t *testing.T) {
	repo := NewUserRepository(testDB)
	u1 := insertTestUser(t, "lu_s1", "sortuser1")
	u2 := insertTestUser(t, "lu_s2", "sortuser2")
	defer cleanupUser(t, u1.ID)
	defer cleanupUser(t, u2.ID)

	// Sort by createdAt ASC / DESC
	users, err := repo.ListUsers(model.UserListFilter{Sort: "+createdAt", Limit: 1})
	require.NoError(t, err)
	assert.Len(t, users, 1)

	users, err = repo.ListUsers(model.UserListFilter{Sort: "-createdAt", Limit: 1})
	require.NoError(t, err)
	assert.Len(t, users, 1)

	// Sort by updatedAt
	users, err = repo.ListUsers(model.UserListFilter{Sort: "+updatedAt", Limit: 100})
	require.NoError(t, err)
	assert.NotEmpty(t, users)

	// Sort by -updatedAt
	users, err = repo.ListUsers(model.UserListFilter{Sort: "-updatedAt", Limit: 100})
	require.NoError(t, err)
	assert.NotEmpty(t, users)

	// Sort by followersCount
	users, err = repo.ListUsers(model.UserListFilter{Sort: "+followersCount", Limit: 100})
	require.NoError(t, err)
	assert.NotEmpty(t, users)

	users, err = repo.ListUsers(model.UserListFilter{Sort: "-followersCount", Limit: 100})
	require.NoError(t, err)
	assert.NotEmpty(t, users)

	// Pagination offset
	users, err = repo.ListUsers(model.UserListFilter{Limit: 100, Offset: 1})
	require.NoError(t, err)
	assert.NotEmpty(t, users)
}

func TestUserRepository_ListUsers_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := NewUserRepository(testDB.WithContext(ctx))
	_, err := repo.ListUsers(model.UserListFilter{})
	assert.Error(t, err)
}

func TestUserRepository_ListUsers_RemoteOrigin(t *testing.T) {
	repo := NewUserRepository(testDB)
	users, err := repo.ListUsers(model.UserListFilter{Origin: "remote", Limit: 10})
	require.NoError(t, err)
	// リモートユーザーがいなくても空配列で返る
	assert.NotNil(t, users)
}

func TestUserRepository_ListUsers_AliveState(t *testing.T) {
	repo := NewUserRepository(testDB)
	users, err := repo.ListUsers(model.UserListFilter{State: "alive", Limit: 10})
	require.NoError(t, err)
	for _, u := range users {
		assert.False(t, u.IsSuspended)
	}
}

func TestUserRepository_ListUsers_LimitCap(t *testing.T) {
	repo := NewUserRepository(testDB)
	// limit > 100 は 100 にキャップされる
	users, err := repo.ListUsers(model.UserListFilter{Limit: 999})
	require.NoError(t, err)
	assert.LessOrEqual(t, len(users), 100)
}

func TestUserRepository_ListRemoteInboxes(t *testing.T) {
	repo := NewUserRepository(testDB)

	// ローカルユーザー (inbox なし) は含まれない。
	local := insertTestUser(t, "lri_local", "lri_local")
	defer cleanupUser(t, local.ID)

	// リモートユーザー A: sharedInbox あり → sharedInbox が使われる。
	hostA := "remote-a.example"
	inboxA := "https://remote-a.example/users/a/inbox"
	sharedA := "https://remote-a.example/inbox"
	a := &model.User{
		ID:                "lri_a",
		Username:          "lri_a",
		UsernameLower:     "lri_a",
		Host:              &hostA,
		Inbox:             &inboxA,
		SharedInbox:       &sharedA,
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}
	require.NoError(t, repo.Create(a))
	defer cleanupUser(t, a.ID)

	// リモートユーザー B: sharedInbox なし → inbox が使われる。
	hostB := "remote-b.example"
	inboxB := "https://remote-b.example/users/b/inbox"
	b := &model.User{
		ID:                "lri_b",
		Username:          "lri_b",
		UsernameLower:     "lri_b",
		Host:              &hostB,
		Inbox:             &inboxB,
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}
	require.NoError(t, repo.Create(b))
	defer cleanupUser(t, b.ID)

	// リモートユーザー C: A と同じ sharedInbox → dedup される。
	c := &model.User{
		ID:                "lri_c",
		Username:          "lri_c",
		UsernameLower:     "lri_c",
		Host:              &hostA,
		SharedInbox:       &sharedA,
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}
	require.NoError(t, repo.Create(c))
	defer cleanupUser(t, c.ID)

	// リモートユーザー D: inbox も sharedInbox も空 → スキップされる。
	hostD := "remote-d.example"
	d := &model.User{
		ID:                "lri_d",
		Username:          "lri_d",
		UsernameLower:     "lri_d",
		Host:              &hostD,
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}
	require.NoError(t, repo.Create(d))
	defer cleanupUser(t, d.ID)

	inboxes, err := repo.ListRemoteInboxes()
	require.NoError(t, err)

	// sharedA と inboxB が含まれる (localは host=NULL で除外、D は空でスキップ、
	// C は A と同じ sharedInbox なので dedup)。
	assert.Contains(t, inboxes, sharedA)
	assert.Contains(t, inboxes, inboxB)
	// inboxA は shared が優先されるので出ない。
	assert.NotContains(t, inboxes, inboxA)
	// dedup 確認: sharedA は 1 回だけ。
	seen := 0
	for _, ib := range inboxes {
		if ib == sharedA {
			seen++
		}
	}
	assert.Equal(t, 1, seen)
}
