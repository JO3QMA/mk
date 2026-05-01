package users

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListsGetMemberships(t *testing.T) {
	h, _ := newTestHandler(t)
	// 認証は router.RequireAuth が 401 を返す責務。ここでは userId 欠落のみ検証。
	assert.Equal(t, http.StatusBadRequest, postStub(h.ListsGetMemberships, `{}`, &model.User{ID: "u1"}).Code)
}

func TestListsGetMemberships_ReturnsOwnedLists(t *testing.T) {
	h, _ := newTestHandler(t)
	repo := testutil.NewMockUserListRepository()
	require.NoError(t, repo.Create(&model.UserList{ID: "l1", UserID: "owner", Name: "favs"}))
	require.NoError(t, repo.Create(&model.UserList{ID: "l2", UserID: "other", Name: "other-favs"}))
	require.NoError(t, repo.AddMember(&model.UserListMembership{UserListID: "l1", UserID: "u2"}))
	require.NoError(t, repo.AddMember(&model.UserListMembership{UserListID: "l2", UserID: "u2"}))
	h.SetUserListRepo(repo)

	rec := postStub(h.ListsGetMemberships, `{"userId":"u2"}`, &model.User{ID: "owner"})
	assert.Equal(t, http.StatusOK, rec.Code)
	var rows []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rows))
	require.Len(t, rows, 1, "only caller-owned lists should be returned")
	assert.Equal(t, "l1", rows[0]["id"])
}

// --- ListsCreateFromPublic ---

func TestListsCreateFromPublic_InvalidParam(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := postStub(h.ListsCreateFromPublic, `{}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- ListsFavorite ---

func TestListsFavorite(t *testing.T) {
	h, _ := newTestHandler(t)
	// listId未指定 → 400
	rec := postStub(h.ListsFavorite, `{}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestListsFavorite_WithRepo(t *testing.T) {
	h, _ := newTestHandler(t)
	favRepo := testutil.NewMockUserListFavoriteRepository()
	h.SetUserListFavoriteRepo(favRepo)
	rec := postStub(h.ListsFavorite, `{"listId":"l1"}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusNoContent, rec.Code)
	exists, _ := favRepo.Exists("u1", "l1")
	assert.True(t, exists)
}

func TestListsFavorite_AlreadyFavorited(t *testing.T) {
	h, _ := newTestHandler(t)
	favRepo := testutil.NewMockUserListFavoriteRepository()
	favRepo.Favorites["u1:l1"] = &model.UserListFavorite{ID: "f1", UserID: "u1", UserListID: "l1"}
	h.SetUserListFavoriteRepo(favRepo)
	rec := postStub(h.ListsFavorite, `{"listId":"l1"}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestListsFavorite_MissingListID(t *testing.T) {
	h, _ := newTestHandler(t)
	favRepo := testutil.NewMockUserListFavoriteRepository()
	h.SetUserListFavoriteRepo(favRepo)
	rec := postStub(h.ListsFavorite, `{}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- ListsUnfavorite ---

func TestListsUnfavorite(t *testing.T) {
	h, _ := newTestHandler(t)
	// listId未指定 → 400
	rec := postStub(h.ListsUnfavorite, `{}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestListsUnfavorite_WithRepo(t *testing.T) {
	h, _ := newTestHandler(t)
	favRepo := testutil.NewMockUserListFavoriteRepository()
	favRepo.Favorites["u1:l1"] = &model.UserListFavorite{ID: "f1", UserID: "u1", UserListID: "l1"}
	h.SetUserListFavoriteRepo(favRepo)
	rec := postStub(h.ListsUnfavorite, `{"listId":"l1"}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusNoContent, rec.Code)
	exists, _ := favRepo.Exists("u1", "l1")
	assert.False(t, exists)
}

// --- ListsUpdate ---

func TestListsUpdate_MissingListID(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := postStub(h.ListsUpdate, `{}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestListsUpdate_Success(t *testing.T) {
	h, _ := newTestHandler(t)
	listRepo := testutil.NewMockUserListRepository()
	listRepo.Lists["l1"] = &model.UserList{ID: "l1", UserID: "u1", Name: "old"}
	h.SetUserListRepo(listRepo)

	rec := postStub(h.ListsUpdate, `{"listId":"l1","name":"new"}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "new", listRepo.Lists["l1"].Name)
}

func TestListsUpdate_NotFound(t *testing.T) {
	h, _ := newTestHandler(t)
	listRepo := testutil.NewMockUserListRepository()
	h.SetUserListRepo(listRepo)

	rec := postStub(h.ListsUpdate, `{"listId":"ghost"}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestListsUpdate_IsPublic(t *testing.T) {
	h, _ := newTestHandler(t)
	listRepo := testutil.NewMockUserListRepository()
	listRepo.Lists["l1"] = &model.UserList{ID: "l1", UserID: "u1", Name: "test"}
	h.SetUserListRepo(listRepo)

	rec := postStub(h.ListsUpdate, `{"listId":"l1","isPublic":true}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, listRepo.Lists["l1"].IsPublic)
}

func TestListsUpdate_NotOwner(t *testing.T) {
	h, _ := newTestHandler(t)
	listRepo := testutil.NewMockUserListRepository()
	listRepo.Lists["l1"] = &model.UserList{ID: "l1", UserID: "other", Name: "test"}
	h.SetUserListRepo(listRepo)

	// u1は所有者ではないので404
	rec := postStub(h.ListsUpdate, `{"listId":"l1","name":"hacked"}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestListsUpdate_NilRepo(t *testing.T) {
	h, _ := newTestHandler(t)
	// userListRepo未注入時はgraceful NoContent
	rec := postStub(h.ListsUpdate, `{"listId":"l1"}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

// --- ListsUpdateMembership ---

func TestListsUpdateMembership_MissingParams(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := postStub(h.ListsUpdateMembership, `{}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestListsUpdateMembership_Success(t *testing.T) {
	h, _ := newTestHandler(t)
	listRepo := testutil.NewMockUserListRepository()
	listRepo.Lists["l1"] = &model.UserList{ID: "l1", UserID: "u1", Name: "test"}
	listRepo.Members = append(listRepo.Members, &model.UserListMembership{ID: "m1", UserListID: "l1", UserID: "u2"})
	h.SetUserListRepo(listRepo)

	rec := postStub(h.ListsUpdateMembership, `{"listId":"l1","userId":"u2","withReplies":true}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestListsUpdateMembership_NotFound(t *testing.T) {
	h, _ := newTestHandler(t)
	listRepo := testutil.NewMockUserListRepository()
	h.SetUserListRepo(listRepo)

	rec := postStub(h.ListsUpdateMembership, `{"listId":"ghost","userId":"u2","withReplies":false}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestListsUpdateMembership_NotOwner(t *testing.T) {
	h, _ := newTestHandler(t)
	listRepo := testutil.NewMockUserListRepository()
	listRepo.Lists["l1"] = &model.UserList{ID: "l1", UserID: "other", Name: "test"}
	h.SetUserListRepo(listRepo)

	rec := postStub(h.ListsUpdateMembership, `{"listId":"l1","userId":"u2","withReplies":true}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestListsUpdateMembership_NilRepo(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := postStub(h.ListsUpdateMembership, `{"listId":"l1","userId":"u2","withReplies":true}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestListsUpdateMembership_MemberNotFound(t *testing.T) {
	h, _ := newTestHandler(t)
	listRepo := testutil.NewMockUserListRepository()
	listRepo.Lists["l1"] = &model.UserList{ID: "l1", UserID: "u1", Name: "test"}
	// メンバーが存在しないためUpdateMembershipがErrNotFoundを返す→404
	h.SetUserListRepo(listRepo)

	rec := postStub(h.ListsUpdateMembership, `{"listId":"l1","userId":"ghost","withReplies":true}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
