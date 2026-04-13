package users

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/server/middleware"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func postStub(handler func(echo.Context) error, body string, user *model.User) *httptest.ResponseRecorder {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if user != nil {
		c.Set(string(middleware.UserContextKey), user)
	}
	_ = handler(c)
	return rec
}

// --- Achievements ---

func TestAchievements_Success(t *testing.T) {
	h, repo := newTestHandler(t)
	repo.Users["u1"] = &model.User{ID: "u1", Username: "alice"}
	repo.Profiles["u1"] = &model.UserProfile{
		UserID:       "u1",
		Achievements: datatypes.JSON(`[{"name":"notes1","unlockedAt":1000}]`),
	}
	rec := postStub(h.Achievements, `{"userId":"u1"}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "notes1")
}

func TestAchievements_NoProfile(t *testing.T) {
	h, repo := newTestHandler(t)
	repo.Users["u1"] = &model.User{ID: "u1", Username: "alice"}
	rec := postStub(h.Achievements, `{"userId":"u1"}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "[]\n", rec.Body.String())
}

func TestAchievements_NotFound(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := postStub(h.Achievements, `{"userId":"ghost"}`, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAchievements_InvalidParam(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := postStub(h.Achievements, `{}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Simple empty array endpoints ---

func TestClips(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := postStub(h.Clips, `{}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestFlashs(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := postStub(h.Flashs, `{}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestGalleryPosts(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := postStub(h.GalleryPosts, `{}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestPages(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := postStub(h.Pages, `{}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestGetFrequentlyRepliedUsers(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := postStub(h.GetFrequentlyRepliedUsers, `{}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestGetFollowingUsersByBirthday(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := postStub(h.GetFollowingUsersByBirthday, `{}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestUserRecommendation(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := postStub(h.UserRecommendation, `{}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestListsGetMemberships(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := postStub(h.ListsGetMemberships, `{}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- NoContent endpoints ---

func TestListsCreateFromPublic(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := postStub(h.ListsCreateFromPublic, `{}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestListsFavorite(t *testing.T) {
	h, _ := newTestHandler(t)
	// listId未指定 → 400
	rec := postStub(h.ListsFavorite, `{}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestListsUnfavorite(t *testing.T) {
	h, _ := newTestHandler(t)
	// listId未指定 → 400
	rec := postStub(h.ListsUnfavorite, `{}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestListsUpdate(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := postStub(h.ListsUpdate, `{}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestListsUpdateMembership(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := postStub(h.ListsUpdateMembership, `{}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

// --- UsersBulk ---

func TestUsersBulk_Success(t *testing.T) {
	h, repo := newTestHandler(t)
	repo.Users["u1"] = &model.User{ID: "u1", Username: "alice", AvatarDecorations: datatypes.JSON("[]")}
	rec := postStub(h.UsersBulk, `{"userIds":["u1"]}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "alice")
}

func TestUsersBulk_Empty(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := postStub(h.UsersBulk, `{"userIds":[]}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestUsersBulk_InvalidParam(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := postStub(h.UsersBulk, `invalid`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- ListsFavorite with repo ---

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

// --- Show with isFollowing ---

func TestShow_IsFollowing(t *testing.T) {
	h, repo := newTestHandler(t)
	repo.Users["u1"] = &model.User{ID: "u1", Username: "alice", AvatarDecorations: datatypes.JSON("[]")}
	repo.Users["u2"] = &model.User{ID: "u2", Username: "bob", AvatarDecorations: datatypes.JSON("[]")}
	fRepo := h.followingRepo
	_ = fRepo // followingRepoがセットされていない場合のテスト

	rec := postStub(h.Show, `{"userId":"u2"}`, &model.User{ID: "u1"})
	require.Equal(t, http.StatusOK, rec.Code)
}
