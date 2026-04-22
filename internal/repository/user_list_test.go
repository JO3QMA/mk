package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cleanupUserList(t *testing.T, id string) {
	t.Helper()
	testDB.Exec(`DELETE FROM "user_list_membership" WHERE "userListId" = ?`, id)
	testDB.Exec(`DELETE FROM "user_list" WHERE id = ?`, id)
}

func TestUserListRepository_CRUD(t *testing.T) {
	repo := NewUserListRepository(testDB)
	createTestUser(t, "ul_owner")

	list := &model.UserList{ID: "ul_1", UserID: "ul_owner", Name: "My List"}
	require.NoError(t, repo.Create(list))
	defer cleanupUserList(t, list.ID)

	found, err := repo.FindByID(list.ID)
	require.NoError(t, err)
	assert.Equal(t, "My List", found.Name)

	lists, err := repo.ListByUser("ul_owner")
	require.NoError(t, err)
	assert.Len(t, lists, 1)

	require.NoError(t, repo.Delete(list.ID))
	_, err = repo.FindByID(list.ID)
	assert.Error(t, err)
}

func TestUserListRepository_FindByID_NotFound(t *testing.T) {
	repo := NewUserListRepository(testDB)
	_, err := repo.FindByID("ghost")
	assert.Error(t, err)
}

func TestUserListRepository_Members(t *testing.T) {
	repo := NewUserListRepository(testDB)
	createTestUser(t, "ul_o2")
	createTestUser(t, "ul_m1")

	list := &model.UserList{ID: "ul_2", UserID: "ul_o2", Name: "Test"}
	require.NoError(t, repo.Create(list))
	defer cleanupUserList(t, list.ID)

	m := &model.UserListMembership{ID: "ulm_1", UserListID: list.ID, UserID: "ul_m1"}
	require.NoError(t, repo.AddMember(m))

	members, err := repo.ListMembers(list.ID)
	require.NoError(t, err)
	assert.Len(t, members, 1)

	require.NoError(t, repo.RemoveMember(list.ID, "ul_m1"))
	members, _ = repo.ListMembers(list.ID)
	assert.Empty(t, members)
}

func TestUserListRepository_ListByUser_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := NewUserListRepository(testDB.WithContext(ctx))
	_, err := repo.ListByUser("x")
	assert.Error(t, err)
}

func TestUserListRepository_ListMembers_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := NewUserListRepository(testDB.WithContext(ctx))
	_, err := repo.ListMembers("x")
	assert.Error(t, err)
}

// TestUserListRepository_AddMember_Duplicate verifies the (userListId, userId)
// unique-constraint violation is mapped to ErrUserListDuplicateMember (#396),
// so the API layer can return the TS-compat ALREADY_ADDED error.
func TestUserListRepository_AddMember_Duplicate(t *testing.T) {
	repo := NewUserListRepository(testDB)
	createTestUser(t, "ul_o3")
	createTestUser(t, "ul_m3")

	list := &model.UserList{ID: "ul_3", UserID: "ul_o3", Name: "Dup"}
	require.NoError(t, repo.Create(list))
	defer cleanupUserList(t, list.ID)

	first := &model.UserListMembership{ID: "ulm_3a", UserListID: list.ID, UserID: "ul_m3"}
	require.NoError(t, repo.AddMember(first))

	dup := &model.UserListMembership{ID: "ulm_3b", UserListID: list.ID, UserID: "ul_m3"}
	err := repo.AddMember(dup)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUserListDuplicateMember),
		"既 member の AddMember は ErrUserListDuplicateMember を返すこと, got: %v", err)
}
