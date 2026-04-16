package users_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/users"
	corefollowing "github.com/shiroha-a/mk/internal/core/following"
	coreuser "github.com/shiroha-a/mk/internal/core/user"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/server/middleware"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// p189Kit bundles the handler and every mock the P4-7 endpoints touch so a
// single test can populate just what it needs.
type p189Kit struct {
	h        *users.Handler
	userRepo *testutil.MockUserRepository
	noteRepo *testutil.MockNoteRepository
	fRepo    *testutil.MockFollowingRepository
	listRepo *testutil.MockUserListRepository
}

func newP189Kit(t *testing.T) *p189Kit {
	t.Helper()
	userRepo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	piningRepo := testutil.NewMockUserNotePiningRepository()
	fRepo := testutil.NewMockFollowingRepository()
	frRepo := testutil.NewMockFollowRequestRepository()
	listRepo := testutil.NewMockUserListRepository()
	idGen, _ := id.NewGenerator("aidx")
	userSvc := coreuser.NewService(userRepo, noteRepo, piningRepo, idGen)
	fSvc := corefollowing.NewService(userRepo, fRepo, frRepo, idGen)
	h := users.NewHandler(userSvc, fSvc, noteRepo, idGen)
	h.SetFollowingRepo(fRepo)
	h.SetUserListRepo(listRepo)
	return &p189Kit{h: h, userRepo: userRepo, noteRepo: noteRepo, fRepo: fRepo, listRepo: listRepo}
}

func postP189(handler func(echo.Context) error, body string, user *model.User) *httptest.ResponseRecorder {
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

// --- GetFrequentlyRepliedUsers ---

func TestGetFrequentlyRepliedUsers_AggregatesAndWeights(t *testing.T) {
	k := newP189Kit(t)
	k.userRepo.Users["me"] = &model.User{ID: "me", Username: "me"}
	k.userRepo.Users["t1"] = &model.User{ID: "t1", Username: "t1"}
	k.userRepo.Users["t2"] = &model.User{ID: "t2", Username: "t2"}
	// me → t1 を 3 件, me → t2 を 1 件の reply を作成する
	ridDummy := "anyReply"
	for i, target := range []string{"t1", "t1", "t1", "t2"} {
		uid := target
		nid := "n" + string(rune('a'+i))
		k.noteRepo.Notes[nid] = &model.Note{
			ID:          nid,
			UserID:      "me",
			ReplyID:     &ridDummy,
			ReplyUserID: &uid,
		}
	}
	rec := postP189(k.h.GetFrequentlyRepliedUsers, `{"userId":"me","limit":10}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)

	var out []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out, 2)
	// 先頭は t1 (count=3, weight=1.0)、次に t2 (count=1, weight=1/3)。
	assert.Equal(t, 1.0, out[0]["weight"])
	assert.InDelta(t, 1.0/3.0, out[1]["weight"], 1e-9)
}

func TestGetFrequentlyRepliedUsers_MissingUserId(t *testing.T) {
	k := newP189Kit(t)
	rec := postP189(k.h.GetFrequentlyRepliedUsers, `{}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetFrequentlyRepliedUsers_UserNotFound(t *testing.T) {
	k := newP189Kit(t)
	rec := postP189(k.h.GetFrequentlyRepliedUsers, `{"userId":"ghost"}`, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// --- GetFollowingUsersByBirthday ---

func TestGetFollowingUsersByBirthday_SingleDayMatch(t *testing.T) {
	k := newP189Kit(t)
	k.userRepo.Users["me"] = &model.User{ID: "me"}
	k.userRepo.Users["fe1"] = &model.User{ID: "fe1", Username: "fe1"}
	k.userRepo.Users["fe2"] = &model.User{ID: "fe2", Username: "fe2"}
	k.fRepo.Followings["f1"] = &model.Following{ID: "f1", FollowerID: "me", FolloweeID: "fe1"}
	k.fRepo.Followings["f2"] = &model.Following{ID: "f2", FollowerID: "me", FolloweeID: "fe2"}
	k.fRepo.Birthdays["fe1"] = "1990-05-10"
	k.fRepo.Birthdays["fe2"] = "1991-07-01"

	rec := postP189(k.h.GetFollowingUsersByBirthday, `{"birthday":{"month":5,"day":10}}`, &model.User{ID: "me"})
	assert.Equal(t, http.StatusOK, rec.Code)
	var out []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out, 1)
	assert.Equal(t, "fe1", out[0]["id"])
}

func TestGetFollowingUsersByBirthday_RangeWrap(t *testing.T) {
	k := newP189Kit(t)
	k.userRepo.Users["me"] = &model.User{ID: "me"}
	k.userRepo.Users["fe1"] = &model.User{ID: "fe1", Username: "fe1"}
	k.userRepo.Users["fe2"] = &model.User{ID: "fe2", Username: "fe2"}
	k.userRepo.Users["fe3"] = &model.User{ID: "fe3", Username: "fe3"}
	k.fRepo.Followings["f1"] = &model.Following{ID: "f1", FollowerID: "me", FolloweeID: "fe1"}
	k.fRepo.Followings["f2"] = &model.Following{ID: "f2", FollowerID: "me", FolloweeID: "fe2"}
	k.fRepo.Followings["f3"] = &model.Following{ID: "f3", FollowerID: "me", FolloweeID: "fe3"}
	k.fRepo.Birthdays["fe1"] = "1990-12-30"
	k.fRepo.Birthdays["fe2"] = "1991-01-03"
	k.fRepo.Birthdays["fe3"] = "1992-06-15"

	rec := postP189(k.h.GetFollowingUsersByBirthday,
		`{"birthday":{"begin":{"month":12,"day":25},"end":{"month":1,"day":5}}}`,
		&model.User{ID: "me"})
	assert.Equal(t, http.StatusOK, rec.Code)
	var out []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out, 2) // fe1 と fe2 のみ
}

func TestGetFollowingUsersByBirthday_BadParams(t *testing.T) {
	k := newP189Kit(t)
	// birthday 欠落
	assert.Equal(t, http.StatusBadRequest, postP189(k.h.GetFollowingUsersByBirthday, `{}`, &model.User{ID: "me"}).Code)
	// month/day のみで month が範囲外
	assert.Equal(t, http.StatusBadRequest, postP189(k.h.GetFollowingUsersByBirthday, `{"birthday":{"month":0,"day":1}}`, &model.User{ID: "me"}).Code)
	// begin は有効だが end の月が範囲外
	assert.Equal(t, http.StatusBadRequest, postP189(k.h.GetFollowingUsersByBirthday, `{"birthday":{"begin":{"month":1,"day":1},"end":{"month":13,"day":1}}}`, &model.User{ID: "me"}).Code)
}

// --- UserRecommendation ---

func TestUserRecommendation_SortedByFollowers(t *testing.T) {
	k := newP189Kit(t)
	now := time.Now()
	// me (自分), r1/r2 (local active, 候補), r3 (remote, 除外),
	// r4 (locked, 除外), r5 (non-explorable, 除外)
	k.userRepo.Users["me"] = &model.User{ID: "me"}
	k.userRepo.Users["r1"] = &model.User{ID: "r1", IsExplorable: true, UpdatedAt: &now, FollowersCount: 5}
	k.userRepo.Users["r2"] = &model.User{ID: "r2", IsExplorable: true, UpdatedAt: &now, FollowersCount: 20}
	remote := "remote.example"
	k.userRepo.Users["r3"] = &model.User{ID: "r3", IsExplorable: true, UpdatedAt: &now, FollowersCount: 999, Host: &remote}
	k.userRepo.Users["r4"] = &model.User{ID: "r4", IsExplorable: true, UpdatedAt: &now, FollowersCount: 999, IsLocked: true}
	k.userRepo.Users["r5"] = &model.User{ID: "r5", IsExplorable: false, UpdatedAt: &now, FollowersCount: 999}

	rec := postP189(k.h.UserRecommendation, `{"limit":10}`, &model.User{ID: "me"})
	assert.Equal(t, http.StatusOK, rec.Code)
	var out []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out, 2)
	// followersCount DESC
	assert.Equal(t, "r2", out[0]["id"])
	assert.Equal(t, "r1", out[1]["id"])
}

func TestUserRecommendation_ExcludesAlreadyFollowed(t *testing.T) {
	k := newP189Kit(t)
	now := time.Now()
	k.userRepo.Users["me"] = &model.User{ID: "me"}
	k.userRepo.Users["r1"] = &model.User{ID: "r1", IsExplorable: true, UpdatedAt: &now, FollowersCount: 5}
	k.userRepo.Users["r2"] = &model.User{ID: "r2", IsExplorable: true, UpdatedAt: &now, FollowersCount: 20}
	k.userRepo.RecommendationFollowing["me"] = []string{"r2"}

	rec := postP189(k.h.UserRecommendation, `{}`, &model.User{ID: "me"})
	assert.Equal(t, http.StatusOK, rec.Code)
	var out []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out, 1)
	assert.Equal(t, "r1", out[0]["id"])
}

// --- ListsCreateFromPublic ---

func TestListsCreateFromPublic_CopiesMembers(t *testing.T) {
	k := newP189Kit(t)
	src := &model.UserList{ID: "src", UserID: "author", Name: "orig", IsPublic: true}
	k.listRepo.Lists[src.ID] = src
	k.listRepo.Members = append(k.listRepo.Members,
		&model.UserListMembership{ID: "m1", UserListID: src.ID, UserID: "u1"},
		&model.UserListMembership{ID: "m2", UserListID: src.ID, UserID: "u2"},
	)

	rec := postP189(k.h.ListsCreateFromPublic, `{"listId":"src","name":"mine"}`, &model.User{ID: "me"})
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp model.UserList
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "me", resp.UserID)
	assert.Equal(t, "mine", resp.Name)
	assert.False(t, resp.IsPublic)

	// 新 list に u1, u2 が追加されているはず。
	copied := 0
	for _, m := range k.listRepo.Members {
		if m.UserListID == resp.ID {
			copied++
		}
	}
	assert.Equal(t, 2, copied)
}

func TestListsCreateFromPublic_NonPublicRejected(t *testing.T) {
	k := newP189Kit(t)
	src := &model.UserList{ID: "src", UserID: "author", Name: "orig", IsPublic: false}
	k.listRepo.Lists[src.ID] = src
	rec := postP189(k.h.ListsCreateFromPublic, `{"listId":"src","name":"mine"}`, &model.User{ID: "me"})
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestListsCreateFromPublic_InvalidParam(t *testing.T) {
	k := newP189Kit(t)
	assert.Equal(t, http.StatusBadRequest, postP189(k.h.ListsCreateFromPublic, `{}`, &model.User{ID: "me"}).Code)
}
