package users

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	corefollowing "github.com/shiroha-a/mk/internal/core/following"
	coreuser "github.com/shiroha-a/mk/internal/core/user"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/server/middleware"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func newTestHandler(t *testing.T) (*Handler, *testutil.MockUserRepository) {
	t.Helper()
	userRepo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	piningRepo := testutil.NewMockUserNotePiningRepository()
	fRepo := testutil.NewMockFollowingRepository()
	frRepo := testutil.NewMockFollowRequestRepository()
	idGen, _ := id.NewGenerator("aidx")
	svc := coreuser.NewService(userRepo, noteRepo, piningRepo, idGen)
	fSvc := corefollowing.NewService(userRepo, fRepo, frRepo, idGen)
	h := NewHandler(svc, fSvc, noteRepo, idGen)
	return h, userRepo
}

func addTestUser(repo *testutil.MockUserRepository) *model.User {
	name := "Test User"
	user := &model.User{
		ID:                "user1",
		Username:          "testuser",
		UsernameLower:     "testuser",
		Name:              &name,
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}
	repo.Users["user1"] = user
	return user
}

func TestShow_ByUserID(t *testing.T) {
	h, userRepo := newTestHandler(t)
	addTestUser(userRepo)

	body := `{"userId": "user1"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/users/show", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Show(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "user1", resp["id"])
	assert.Equal(t, "testuser", resp["username"])
	assert.Equal(t, "Test User", resp["name"])
}

// stubChartHook captures users handler chart fires.
type stubChartHook struct {
	calls []struct {
		ownerID, viewerID, visitor string
	}
}

func (s *stubChartHook) OnUserShow(ownerID, viewerID, visitor string) {
	s.calls = append(s.calls, struct {
		ownerID, viewerID, visitor string
	}{ownerID, viewerID, visitor})
}

func TestShow_FiresChartHookAuthenticated(t *testing.T) {
	h, userRepo := newTestHandler(t)
	addTestUser(userRepo)
	hook := &stubChartHook{}
	h.SetChartHook(hook)

	body := `{"userId": "user1"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/users/show", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("misskeyUser", &model.User{ID: "viewer1"})

	require.NoError(t, h.Show(c))
	require.Len(t, hook.calls, 1)
	assert.Equal(t, "user1", hook.calls[0].ownerID)
	assert.Equal(t, "viewer1", hook.calls[0].viewerID)
	assert.Empty(t, hook.calls[0].visitor)
}

func TestShow_FiresChartHookAnonymous(t *testing.T) {
	h, userRepo := newTestHandler(t)
	addTestUser(userRepo)
	hook := &stubChartHook{}
	h.SetChartHook(hook)

	body := `{"userId": "user1"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/users/show", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, h.Show(c))
	require.Len(t, hook.calls, 1)
	assert.Equal(t, "user1", hook.calls[0].ownerID)
	assert.Equal(t, "", hook.calls[0].viewerID)
	assert.NotEmpty(t, hook.calls[0].visitor) // RemoteAddr is set by httptest
}

func TestShow_ByUsername(t *testing.T) {
	h, userRepo := newTestHandler(t)
	addTestUser(userRepo)

	body := `{"username": "testuser"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/users/show", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Show(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "user1", resp["id"])
}

func TestShow_ByUsernameWithHost(t *testing.T) {
	h, userRepo := newTestHandler(t)

	host := "remote.example.com"
	user := &model.User{
		ID:                "user2",
		Username:          "remoteuser",
		UsernameLower:     "remoteuser",
		Host:              &host,
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}
	userRepo.Users["user2"] = user

	body := `{"username": "remoteuser", "host": "remote.example.com"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/users/show", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Show(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "user2", resp["id"])
}

func TestShow_NotFound(t *testing.T) {
	h, _ := newTestHandler(t)

	body := `{"userId": "nonexistent"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/users/show", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Show(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// stubRemoteResolver is a test double for coreuser.RemoteUserResolver.
type stubRemoteResolver struct {
	user *model.User
	err  error
}

func (s *stubRemoteResolver) ResolveByUsernameHost(_, _ string) (*model.User, error) {
	return s.user, s.err
}

func newTestHandlerWithRemoteResolver(t *testing.T, resolver coreuser.RemoteUserResolver) (*Handler, *testutil.MockUserRepository) {
	t.Helper()
	userRepo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	piningRepo := testutil.NewMockUserNotePiningRepository()
	fRepo := testutil.NewMockFollowingRepository()
	frRepo := testutil.NewMockFollowRequestRepository()
	idGen, _ := id.NewGenerator("aidx")
	svc := coreuser.NewService(userRepo, noteRepo, piningRepo, idGen)
	svc.SetRemoteUserResolver(resolver)
	fSvc := corefollowing.NewService(userRepo, fRepo, frRepo, idGen)
	return NewHandler(svc, fSvc, noteRepo, idGen), userRepo
}

func TestShow_ByUsernameWithHost_RemoteResolveSucceeds(t *testing.T) {
	// webfinger + ResolveActor が成功し、返された remote user がそのまま
	// UserDetailed として返されることを確認する。
	host := "remote.example"
	remoteUser := &model.User{
		ID:                "uR",
		Username:          "remote",
		UsernameLower:     "remote",
		Host:              &host,
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}
	h, _ := newTestHandlerWithRemoteResolver(t, &stubRemoteResolver{user: remoteUser})

	body := `{"username":"remote","host":"remote.example"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/users/show", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, h.Show(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "uR", resp["id"])
}

func TestShow_ByUsernameWithHost_RemoteResolveFails(t *testing.T) {
	// resolver が error を返す場合、FAILED_TO_RESOLVE_REMOTE_USER がレスポンス
	// として返ることを確認する。HTTP ステータス / JSON の code & id を検証。
	h, _ := newTestHandlerWithRemoteResolver(t, &stubRemoteResolver{err: assert.AnError})

	body := `{"username":"ghost","host":"remote.example"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/users/show", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, h.Show(c))
	assert.Equal(t, http.StatusNotFound, rec.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	errObj, ok := resp["error"].(map[string]any)
	require.True(t, ok, "response must have error field")
	assert.Equal(t, "FAILED_TO_RESOLVE_REMOTE_USER", errObj["code"])
	// apierr.UUIDFailedToResolveRemoteUser と一致すること。
	assert.Equal(t, "ef7b9be4-9cba-4e6f-ab41-90ed171c7d3c", errObj["id"])
}

func TestShow_MissingParams(t *testing.T) {
	h, _ := newTestHandler(t)

	body := `{}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/users/show", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Show(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestShow_WithProfile(t *testing.T) {
	h, userRepo := newTestHandler(t)
	addTestUser(userRepo)

	desc := "Hello, I'm a test user"
	location := "Tokyo"
	userRepo.Profiles["user1"] = &model.UserProfile{
		UserID:      "user1",
		Description: &desc,
		Location:    &location,
		Fields:      datatypes.JSON([]byte("[]")),
	}

	body := `{"userId": "user1"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/users/show", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Show(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "Hello, I'm a test user", resp["description"])
	assert.Equal(t, "Tokyo", resp["location"])
}

func TestShow_InvalidJSON(t *testing.T) {
	h, _ := newTestHandler(t)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/users/show", strings.NewReader("{invalid"))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Show(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestShow_UsernameNotFound(t *testing.T) {
	h, _ := newTestHandler(t)

	body := `{"username": "nonexistent"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/users/show", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Show(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestShow_ViewerDependentFields(t *testing.T) {
	h, userRepo := newTestHandler(t)

	// ターゲットユーザー
	target := &model.User{
		ID:                "target1",
		Username:          "target",
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}
	userRepo.Users["target1"] = target

	// viewer
	viewer := &model.User{ID: "viewer1", Username: "viewer"}

	// blockingリポジトリをセット (viewerがtargetをブロック)
	blockingRepo := testutil.NewMockBlockingRepository()
	blockingRepo.Blockings["b1"] = &model.Blocking{ID: "b1", BlockerID: "viewer1", BlockeeID: "target1"}
	h.SetBlockingRepo(blockingRepo)

	// mutingリポジトリをセット
	mutingRepo := testutil.NewMockMutingRepository()
	h.SetMutingRepo(mutingRepo)

	// followRequestリポジトリをセット
	followRequestRepo := testutil.NewMockFollowRequestRepository()
	h.SetFollowRequestRepo(followRequestRepo)

	body := `{"userId": "target1"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/users/show", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(string(middleware.UserContextKey), viewer)

	err := h.Show(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["isBlocking"])
	assert.Equal(t, false, resp["isBlocked"])
	assert.Equal(t, false, resp["isMuted"])
}

func TestShow_ViewerMemo(t *testing.T) {
	h, userRepo := newTestHandler(t)

	target := &model.User{
		ID:                "target1",
		Username:          "target",
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}
	userRepo.Users["target1"] = target

	viewer := &model.User{ID: "viewer1", Username: "viewer"}

	// memoリポジトリをセット
	memoRepo := testutil.NewMockUserMemoRepository()
	memoRepo.Memos["viewer1:target1"] = &model.UserMemo{
		UserID:       "viewer1",
		TargetUserID: "target1",
		Memo:         "important person",
	}
	h.SetMemoRepo(memoRepo)

	// followingリポジトリをセット (notify/withRepliesのテスト用)
	followingRepo := testutil.NewMockFollowingRepository()
	notify := "normal"
	followingRepo.Followings["f1"] = &model.Following{
		ID:          "f1",
		FollowerID:  "viewer1",
		FolloweeID:  "target1",
		Notify:      &notify,
		WithReplies: true,
	}
	h.SetFollowingRepo(followingRepo)

	// renoteMutingリポジトリをセット
	renoteMutingRepo := testutil.NewMockRenoteMutingRepository()
	h.SetRenoteMutingRepo(renoteMutingRepo)

	body := `{"userId": "target1"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/users/show", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(string(middleware.UserContextKey), viewer)

	err := h.Show(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "important person", resp["memo"])
	assert.Equal(t, "normal", resp["notify"])
	assert.Equal(t, true, resp["withReplies"])
	assert.Equal(t, true, resp["isFollowing"])
	assert.Equal(t, false, resp["isRenoteMuted"])
}

func TestShow_RemoteUserInstance(t *testing.T) {
	h, userRepo := newTestHandler(t)

	host := "remote.example.com"
	target := &model.User{
		ID:                "remote1",
		Username:          "remoteuser",
		Host:              &host,
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}
	userRepo.Users["remote1"] = target

	// instanceリポジトリをセット
	instanceRepo := testutil.NewMockInstanceRepository()
	instName := "Remote Instance"
	softwareName := "misskey"
	instanceRepo.Instances["remote.example.com"] = &model.Instance{
		ID:               "inst1",
		Host:             "remote.example.com",
		Name:             &instName,
		SoftwareName:     &softwareName,
		FirstRetrievedAt: time.Now(),
	}
	h.SetInstanceRepo(instanceRepo)

	body := `{"userId": "remote1"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/users/show", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Show(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	inst := resp["instance"].(map[string]any)
	assert.Equal(t, "Remote Instance", inst["name"])
	assert.Equal(t, "misskey", inst["softwareName"])
}

// --- post is a small helper that exercises a handler with an optional body ---

func post(h echo.HandlerFunc, body string) *httptest.ResponseRecorder {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	_ = h(c)
	return rec
}

// --- Search ---

func TestSearch_Success(t *testing.T) {
	h, repo := newTestHandler(t)
	addTestUser(repo)

	rec := post(h.Search, `{"query": "test"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	var out []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Len(t, out, 1)
}

func TestSearch_DefaultLimit(t *testing.T) {
	h, repo := newTestHandler(t)
	addTestUser(repo)

	rec := post(h.Search, `{"query": "test", "limit": 0}`)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestSearch_InvalidJSON(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := post(h.Search, `{invalid`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Notes ---

func TestNotes_Success(t *testing.T) {
	h, repo := newTestHandler(t)
	addTestUser(repo)
	// Insert a note via the noteRepo embedded in handler
	idGen, _ := id.NewGenerator("aidx")
	noteRepo := h.noteRepo.(*testutil.MockNoteRepository)
	noteID := idGen.Generate(time.Now())
	text := "hello"
	noteRepo.Notes[noteID] = &model.Note{
		ID:         noteID,
		UserID:     "user1",
		Text:       &text,
		Visibility: model.NoteVisibilityPublic,
		Reactions:  datatypes.JSON([]byte("{}")),
	}

	rec := post(h.Notes, `{"userId": "user1"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	var out []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Len(t, out, 1)
}

func TestNotes_LimitClamp(t *testing.T) {
	h, repo := newTestHandler(t)
	addTestUser(repo)
	rec := post(h.Notes, `{"userId": "user1", "limit": 9999}`)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestNotes_UserNotFound(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := post(h.Notes, `{"userId": "ghost"}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestNotes_InvalidParam(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := post(h.Notes, `{}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Followers / Following ---

func TestFollowers_Success(t *testing.T) {
	h, repo := newTestHandler(t)
	addTestUser(repo)
	// Add a follower relationship
	repo.Users["follower1"] = &model.User{
		ID:                "follower1",
		Username:          "follower1",
		UsernameLower:     "follower1",
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}
	fSvc := h.followingService
	_, err := fSvc.Follow("follower1", "user1")
	require.NoError(t, err)

	rec := post(h.Followers, `{"userId": "user1"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	var out []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Len(t, out, 1)
}

func TestFollowers_LimitClamp(t *testing.T) {
	h, repo := newTestHandler(t)
	addTestUser(repo)
	rec := post(h.Followers, `{"userId": "user1", "limit": 9999}`)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestFollowers_UserNotFound(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := post(h.Followers, `{"userId": "ghost"}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestFollowers_InvalidParam(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := post(h.Followers, `{}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestFollowing_Success(t *testing.T) {
	h, repo := newTestHandler(t)
	addTestUser(repo)
	repo.Users["followee1"] = &model.User{
		ID:                "followee1",
		Username:          "followee1",
		UsernameLower:     "followee1",
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}
	_, err := h.followingService.Follow("user1", "followee1")
	require.NoError(t, err)

	rec := post(h.Following, `{"userId": "user1"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	var out []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Len(t, out, 1)
}

func TestFollowing_InvalidParam(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := post(h.Following, `{}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestShow_BulkUserIDs(t *testing.T) {
	h, repo := newTestHandler(t)
	repo.Users["u1"] = &model.User{ID: "u1", Username: "alice", AvatarDecorations: datatypes.JSON([]byte("[]"))}
	repo.Users["u2"] = &model.User{ID: "u2", Username: "bob", AvatarDecorations: datatypes.JSON([]byte("[]"))}
	rec := post(h.Show, `{"userIds":["u1","u2","ghost"]}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	var out []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Len(t, out, 2)
}

func TestShow_BulkUserIDs_Empty(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := post(h.Show, `{"userIds":[]}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "[]\n", rec.Body.String())
}

// users/show bulk path は ShowManyByIDs に切り替えた (#503) ので、入力順が
// レスポンスにも保持されることを担保する。
func TestShow_BulkUserIDs_PreservesOrder(t *testing.T) {
	h, repo := newTestHandler(t)
	repo.Users["u1"] = &model.User{ID: "u1", Username: "alice", AvatarDecorations: datatypes.JSON([]byte("[]"))}
	repo.Users["u2"] = &model.User{ID: "u2", Username: "bob", AvatarDecorations: datatypes.JSON([]byte("[]"))}
	repo.Users["u3"] = &model.User{ID: "u3", Username: "carol", AvatarDecorations: datatypes.JSON([]byte("[]"))}

	rec := post(h.Show, `{"userIds":["u3","ghost","u1","u2"]}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	var out []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out, 3)
	assert.Equal(t, "u3", out[0]["id"])
	assert.Equal(t, "u1", out[1]["id"])
	assert.Equal(t, "u2", out[2]["id"])
}

// 100 件を超えた userIds は先頭 100 件に切り捨てる動作を担保する (TS 互換)。
func TestShow_BulkUserIDs_Truncates100(t *testing.T) {
	h, repo := newTestHandler(t)
	ids := make([]string, 0, 105)
	for i := 0; i < 105; i++ {
		uid := fmt.Sprintf("ub%03d", i)
		repo.Users[uid] = &model.User{ID: uid, Username: uid, AvatarDecorations: datatypes.JSON([]byte("[]"))}
		ids = append(ids, fmt.Sprintf("%q", uid))
	}
	body := `{"userIds":[` + strings.Join(ids, ",") + `]}`
	rec := post(h.Show, body)
	assert.Equal(t, http.StatusOK, rec.Code)
	var out []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Len(t, out, 100, "userIds は 100 件で切り捨てられるはず")
}

// --- Internal error paths via failing repos ---

type failingNoteRepo struct {
	*testutil.MockNoteRepository
}

func (f *failingNoteRepo) ListByUserID(_ string, _, _ string, _ int) ([]*model.Note, error) {
	return nil, assertErr
}

type failingUserRepo struct {
	*testutil.MockUserRepository
}

func (f *failingUserRepo) SearchByUsername(_ string, _, _ int) ([]*model.User, error) {
	return nil, assertErr
}

type failingFollowingRepo struct {
	*testutil.MockFollowingRepository
}

func (f *failingFollowingRepo) ListFollowers(_ string, _, _ int) ([]*model.Following, error) {
	return nil, assertErr
}

func (f *failingFollowingRepo) ListFollowing(_ string, _, _ int) ([]*model.Following, error) {
	return nil, assertErr
}

var assertErr = &simpleErr{"stub"}

type simpleErr struct{ msg string }

func (e *simpleErr) Error() string { return e.msg }

func newHandlerWithFailingNoteRepo(t *testing.T) *Handler {
	t.Helper()
	userRepo := testutil.NewMockUserRepository()
	noteRepo := &failingNoteRepo{MockNoteRepository: testutil.NewMockNoteRepository()}
	piningRepo := testutil.NewMockUserNotePiningRepository()
	fRepo := testutil.NewMockFollowingRepository()
	frRepo := testutil.NewMockFollowRequestRepository()
	idGen, _ := id.NewGenerator("aidx")
	svc := coreuser.NewService(userRepo, noteRepo, piningRepo, idGen)
	fSvc := corefollowing.NewService(userRepo, fRepo, frRepo, idGen)
	addTestUser(userRepo)
	return NewHandler(svc, fSvc, noteRepo, idGen)
}

func newHandlerWithFailingSearch(t *testing.T) *Handler {
	t.Helper()
	mockUR := testutil.NewMockUserRepository()
	addTestUser(mockUR)
	userRepo := &failingUserRepo{MockUserRepository: mockUR}
	noteRepo := testutil.NewMockNoteRepository()
	piningRepo := testutil.NewMockUserNotePiningRepository()
	fRepo := testutil.NewMockFollowingRepository()
	frRepo := testutil.NewMockFollowRequestRepository()
	idGen, _ := id.NewGenerator("aidx")
	svc := coreuser.NewService(userRepo, noteRepo, piningRepo, idGen)
	fSvc := corefollowing.NewService(userRepo, fRepo, frRepo, idGen)
	return NewHandler(svc, fSvc, noteRepo, idGen)
}

func newHandlerWithFailingFollowing(t *testing.T) *Handler {
	t.Helper()
	userRepo := testutil.NewMockUserRepository()
	addTestUser(userRepo)
	noteRepo := testutil.NewMockNoteRepository()
	piningRepo := testutil.NewMockUserNotePiningRepository()
	fRepo := &failingFollowingRepo{MockFollowingRepository: testutil.NewMockFollowingRepository()}
	frRepo := testutil.NewMockFollowRequestRepository()
	idGen, _ := id.NewGenerator("aidx")
	svc := coreuser.NewService(userRepo, noteRepo, piningRepo, idGen)
	fSvc := corefollowing.NewService(userRepo, fRepo, frRepo, idGen)
	return NewHandler(svc, fSvc, noteRepo, idGen)
}

func TestSearch_InternalError(t *testing.T) {
	h := newHandlerWithFailingSearch(t)
	rec := post(h.Search, `{"query": "test"}`)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestNotes_InternalError(t *testing.T) {
	h := newHandlerWithFailingNoteRepo(t)
	rec := post(h.Notes, `{"userId": "user1"}`)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestFollowers_InternalError(t *testing.T) {
	h := newHandlerWithFailingFollowing(t)
	rec := post(h.Followers, `{"userId": "user1"}`)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestFollowing_InternalError(t *testing.T) {
	h := newHandlerWithFailingFollowing(t)
	rec := post(h.Following, `{"userId": "user1"}`)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- Phase 7-3 (#245): pinnedNotes / pinnedPage on users/show ---

type stubPageRepoForPin struct {
	page *model.Page
}

func (s *stubPageRepoForPin) Create(*model.Page) error                              { return nil }
func (s *stubPageRepoForPin) FindByID(string) (*model.Page, error)                  { return s.page, nil }
func (s *stubPageRepoForPin) FindByUserAndName(string, string) (*model.Page, error) { return nil, nil }
func (s *stubPageRepoForPin) UpdateFields(string, map[string]any) error             { return nil }
func (s *stubPageRepoForPin) Delete(*model.Page) error                              { return nil }
func (s *stubPageRepoForPin) ListByUser(string, string, string, int, int) ([]*model.Page, error) {
	return nil, nil
}
func (s *stubPageRepoForPin) ListPublicByUser(string, string, string, int, int) ([]*model.Page, error) {
	return nil, nil
}
func (s *stubPageRepoForPin) ListFeatured(string, string, int, int) ([]*model.Page, error) {
	return nil, nil
}
func (s *stubPageRepoForPin) IncrementCount(string, string, int) error { return nil }

func TestShow_PinnedNotes_Populated(t *testing.T) {
	h, userRepo := newTestHandler(t)
	addTestUser(userRepo)

	piningRepo := testutil.NewMockUserNotePiningRepository()
	require.NoError(t, piningRepo.Create(&model.UserNotePining{ID: "p1", UserID: "user1", NoteID: "note_a"}))
	h.SetPiningRepo(piningRepo)

	// note 本体は h.noteRepo (newTestHandler 内で MockNoteRepo) に入れる
	nr, _ := h.noteRepo.(*testutil.MockNoteRepository)
	require.NotNil(t, nr)
	txt := "pinned!"
	nr.Notes["note_a"] = &model.Note{ID: "note_a", UserID: "user1", Text: &txt, Visibility: model.NoteVisibilityPublic, Reactions: datatypes.JSON([]byte("{}"))}

	body := `{"userId": "user1"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/users/show", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	require.NoError(t, h.Show(c))

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	ids, ok := resp["pinnedNoteIds"].([]any)
	require.True(t, ok)
	assert.Len(t, ids, 1)
	assert.Equal(t, "note_a", ids[0])
	notes, ok := resp["pinnedNotes"].([]any)
	require.True(t, ok)
	assert.Len(t, notes, 1)
}

func TestShow_PinnedPage_Populated(t *testing.T) {
	h, userRepo := newTestHandler(t)
	addTestUser(userRepo)

	pageID := "pg_1"
	userRepo.Profiles["user1"] = &model.UserProfile{
		UserID:       "user1",
		Fields:       datatypes.JSON([]byte("[]")),
		PinnedPageID: &pageID,
	}
	h.SetPageRepo(&stubPageRepoForPin{page: &model.Page{ID: pageID, Title: "my page", UserID: "user1"}})

	body := `{"userId": "user1"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/users/show", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	require.NoError(t, h.Show(c))

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, pageID, resp["pinnedPageId"])
	page, ok := resp["pinnedPage"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "my page", page["title"])
}

func TestShow_PinnedFields_Defaults(t *testing.T) {
	h, userRepo := newTestHandler(t)
	addTestUser(userRepo)

	body := `{"userId": "user1"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/users/show", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	require.NoError(t, h.Show(c))

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	// pining未wireでもリストは存在 (空)
	ids, _ := resp["pinnedNoteIds"].([]any)
	assert.Empty(t, ids)
	assert.Nil(t, resp["pinnedPageId"])
	assert.Nil(t, resp["pinnedPage"])
}
