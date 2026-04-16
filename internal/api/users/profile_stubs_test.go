package users

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubGalleryRepo は users 向けに ListByUser のみ返す最小スタブ。
// Misskey の GalleryRepository は ListByUser / ListLikesByUser の 2 メソッド
// しか持たず、MockGalleryRepository は別パッケージに定義されていないため
// 直接 interface を満たす定義を置く。
type stubGalleryRepo struct {
	posts []*model.GalleryPost
}

func (s *stubGalleryRepo) ListByUser(_ string, _, _ int) ([]*model.GalleryPost, error) {
	return s.posts, nil
}
func (s *stubGalleryRepo) ListLikesByUser(_ string, _, _ int) ([]*model.GalleryLike, error) {
	return nil, nil
}

// --- Clips -------------------------------------------------------------------

func TestClips_FiltersNonPublicForStranger(t *testing.T) {
	h, _ := newTestHandler(t)
	repo := testutil.NewMockClipRepository()
	require.NoError(t, repo.Create(&model.Clip{ID: "c1", UserID: "owner", Name: "pub", IsPublic: true}))
	require.NoError(t, repo.Create(&model.Clip{ID: "c2", UserID: "owner", Name: "priv", IsPublic: false}))
	h.SetClipRepo(repo)

	rec := postStub(h.Clips, `{"userId":"owner"}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	var rows []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rows))
	require.Len(t, rows, 1)
	assert.Equal(t, "c1", rows[0]["id"])
}

func TestClips_OwnerSeesPrivate(t *testing.T) {
	h, _ := newTestHandler(t)
	repo := testutil.NewMockClipRepository()
	require.NoError(t, repo.Create(&model.Clip{ID: "c1", UserID: "owner", IsPublic: true}))
	require.NoError(t, repo.Create(&model.Clip{ID: "c2", UserID: "owner", IsPublic: false}))
	h.SetClipRepo(repo)

	rec := postStub(h.Clips, `{"userId":"owner"}`, &model.User{ID: "owner"})
	var rows []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rows))
	assert.Len(t, rows, 2)
}

// --- Flashs ------------------------------------------------------------------

func TestFlashs_HidesNonPublic(t *testing.T) {
	h, _ := newTestHandler(t)
	repo := testutil.NewMockFlashRepository()
	require.NoError(t, repo.Create(&model.Flash{ID: "f1", UserID: "owner", Visibility: "public"}))
	require.NoError(t, repo.Create(&model.Flash{ID: "f2", UserID: "owner", Visibility: "private"}))
	h.SetFlashRepo(repo)
	rec := postStub(h.Flashs, `{"userId":"owner"}`, nil)
	var rows []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rows))
	require.Len(t, rows, 1)
	assert.Equal(t, "f1", rows[0]["id"])
}

// --- GalleryPosts ------------------------------------------------------------

func TestGalleryPosts_ReturnsAll(t *testing.T) {
	h, _ := newTestHandler(t)
	h.SetGalleryRepo(&stubGalleryRepo{posts: []*model.GalleryPost{
		{ID: "g1", UserID: "owner", Title: "t1"},
		{ID: "g2", UserID: "owner", Title: "t2"},
	}})
	rec := postStub(h.GalleryPosts, `{"userId":"owner"}`, nil)
	var rows []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rows))
	assert.Len(t, rows, 2)
}

// --- Pages -------------------------------------------------------------------

func TestPages_HidesNonPublic(t *testing.T) {
	h, _ := newTestHandler(t)
	repo := testutil.NewMockPageRepository()
	require.NoError(t, repo.Create(&model.Page{ID: "p1", UserID: "owner", Visibility: model.PageVisibilityPublic}))
	require.NoError(t, repo.Create(&model.Page{ID: "p2", UserID: "owner", Visibility: model.PageVisibilityFollowers}))
	h.SetPageRepo(repo)
	rec := postStub(h.Pages, `{"userId":"owner"}`, nil)
	var rows []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rows))
	require.Len(t, rows, 1)
	assert.Equal(t, "p1", rows[0]["id"])
}

// --- ListsGetMemberships -----------------------------------------------------

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

// --- UsersBulk ---------------------------------------------------------------

func TestUsersBulk_LimitsTo100(t *testing.T) {
	h, userRepo := newTestHandler(t)
	// 150 人の user を仕込む
	for i := 0; i < 150; i++ {
		id := "u" + timeSuffix(i)
		userRepo.Users[id] = &model.User{ID: id, Username: id, UsernameLower: id}
	}
	ids := make([]string, 150)
	for i := 0; i < 150; i++ {
		ids[i] = "u" + timeSuffix(i)
	}
	body, _ := json.Marshal(map[string]any{"userIds": ids})
	rec := postStub(h.UsersBulk, string(body), nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	var rows []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rows))
	assert.LessOrEqual(t, len(rows), 100)
}

func timeSuffix(i int) string {
	s := time.Date(2025, 1, 1, 0, 0, i, 0, time.UTC).Format("20060102150405")
	return s
}
