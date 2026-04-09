package admin_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	apiadmin "github.com/shiroha-a/mk/internal/api/admin"
	"github.com/shiroha-a/mk/internal/core/signup"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/server/middleware"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestHandler(t *testing.T) (*apiadmin.Handler, *testutil.MockUserRepository, *testutil.MockMetaRepository) {
	t.Helper()
	userRepo := testutil.NewMockUserRepository()
	metaRepo := testutil.NewMockMetaRepository()
	metaRepo.Meta = &model.Meta{ID: "x"}
	idGen, _ := id.NewGenerator("aidx")
	signupSvc := signup.NewService(userRepo, metaRepo, idGen)
	h := apiadmin.NewHandler(signupSvc, metaRepo, userRepo, idGen)
	return h, userRepo, metaRepo
}

func doPost(h func(echo.Context) error, body string, user *model.User) *httptest.ResponseRecorder {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if user != nil {
		c.Set(string(middleware.UserContextKey), user)
	}
	_ = h(c)
	return rec
}

// --- AccountsCreate ---

func TestAccountsCreate_InitialSetup(t *testing.T) {
	h, _, metaRepo := newTestHandler(t)
	// rootUserId=nil → 初回セットアップ
	rec := doPost(h.AccountsCreate, `{"username":"admin","password":"pass123"}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "admin", resp["username"])
	assert.NotEmpty(t, resp["token"])

	// rootUserId が設定された
	assert.NotNil(t, metaRepo.Meta.RootUserID)
}

func TestAccountsCreate_NotInitialSetup_RequiresAdmin(t *testing.T) {
	h, _, metaRepo := newTestHandler(t)
	rootID := "root1"
	metaRepo.Meta.RootUserID = &rootID

	// 認証なし → ACCESS_DENIED
	rec := doPost(h.AccountsCreate, `{"username":"user2","password":"pass"}`, nil)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestAccountsCreate_AsRootUser(t *testing.T) {
	h, _, metaRepo := newTestHandler(t)
	rootID := "root1"
	metaRepo.Meta.RootUserID = &rootID

	rootUser := &model.User{ID: "root1", Username: "root"}
	rec := doPost(h.AccountsCreate, `{"username":"user2","password":"pass"}`, rootUser)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAccountsCreate_AsNonRoot_Denied(t *testing.T) {
	h, _, metaRepo := newTestHandler(t)
	rootID := "root1"
	metaRepo.Meta.RootUserID = &rootID

	otherUser := &model.User{ID: "other", Username: "other"}
	rec := doPost(h.AccountsCreate, `{"username":"user2","password":"pass"}`, otherUser)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestAccountsCreate_InvalidJSON(t *testing.T) {
	h, _, _ := newTestHandler(t)
	rec := doPost(h.AccountsCreate, `invalid`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAccountsCreate_DuplicateUsername(t *testing.T) {
	h, userRepo, _ := newTestHandler(t)
	userRepo.Users["u1"] = &model.User{ID: "u1", Username: "taken", UsernameLower: "taken"}

	rec := doPost(h.AccountsCreate, `{"username":"taken","password":"pass"}`, nil)
	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestAccountsCreate_MetaFetchError(t *testing.T) {
	h, _, metaRepo := newTestHandler(t)
	metaRepo.Meta = nil // Fetch will error
	rec := doPost(h.AccountsCreate, `{"username":"admin","password":"pass"}`, nil)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- ShowUser ---

func TestShowUser_Success(t *testing.T) {
	h, userRepo, _ := newTestHandler(t)
	userRepo.Users["u1"] = &model.User{ID: "u1", Username: "test"}

	rec := doPost(h.ShowUser, `{"userId":"u1"}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestShowUser_NotFound(t *testing.T) {
	h, _, _ := newTestHandler(t)
	rec := doPost(h.ShowUser, `{"userId":"ghost"}`, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestShowUser_InvalidParam(t *testing.T) {
	h, _, _ := newTestHandler(t)
	rec := doPost(h.ShowUser, `{}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- ShowUsers ---

func TestShowUsers_Success(t *testing.T) {
	h, userRepo, _ := newTestHandler(t)
	userRepo.Users["u1"] = &model.User{ID: "u1", Username: "a"}
	userRepo.Users["u2"] = &model.User{ID: "u2", Username: "b"}

	rec := doPost(h.ShowUsers, `{"limit":10}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp []any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp, 2)
}

func TestShowUsers_WithFilter(t *testing.T) {
	h, userRepo, _ := newTestHandler(t)
	userRepo.Users["u1"] = &model.User{ID: "u1", Username: "a", IsSuspended: true}
	userRepo.Users["u2"] = &model.User{ID: "u2", Username: "b"}

	rec := doPost(h.ShowUsers, `{"state":"suspended","limit":10}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp []any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp, 1)
}

// --- SuspendUser / UnsuspendUser ---

func TestSuspendUser_Success(t *testing.T) {
	h, userRepo, _ := newTestHandler(t)
	userRepo.Users["u1"] = &model.User{ID: "u1", Username: "target"}

	rec := doPost(h.SuspendUser, `{"userId":"u1"}`, nil)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestSuspendUser_NotFound(t *testing.T) {
	h, _, _ := newTestHandler(t)
	rec := doPost(h.SuspendUser, `{"userId":"ghost"}`, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestSuspendUser_InvalidParam(t *testing.T) {
	h, _, _ := newTestHandler(t)
	rec := doPost(h.SuspendUser, `{}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUnsuspendUser_Success(t *testing.T) {
	h, userRepo, _ := newTestHandler(t)
	userRepo.Users["u1"] = &model.User{ID: "u1", IsSuspended: true}

	rec := doPost(h.UnsuspendUser, `{"userId":"u1"}`, nil)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestUnsuspendUser_NotFound(t *testing.T) {
	h, _, _ := newTestHandler(t)
	rec := doPost(h.UnsuspendUser, `{"userId":"ghost"}`, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// --- AdminMeta / UpdateMeta ---

func TestAdminMeta_Success(t *testing.T) {
	h, _, _ := newTestHandler(t)
	rec := doPost(h.AdminMeta, `{}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAdminMeta_FetchError(t *testing.T) {
	h, _, metaRepo := newTestHandler(t)
	metaRepo.Meta = nil
	rec := doPost(h.AdminMeta, `{}`, nil)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestUpdateMeta_Success(t *testing.T) {
	h, _, _ := newTestHandler(t)
	rec := doPost(h.UpdateMeta, `{"name":"My Instance"}`, nil)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestAccountsCreate_EmptyUsername(t *testing.T) {
	h, _, _ := newTestHandler(t)
	rec := doPost(h.AccountsCreate, `{"username":"","password":"pass"}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAccountsCreate_WhitespaceOnlyUsername(t *testing.T) {
	// Bindはusernameがemptyかチェックするが、空白のみはbindを通過する。
	// Signup側でTrimSpace後にemptyになり、ErrInvalidUsernameが返る。
	h, _, _ := newTestHandler(t)
	rec := doPost(h.AccountsCreate, `{"username":"   ","password":"pass"}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestShowUsers_InvalidJSON(t *testing.T) {
	h, _, _ := newTestHandler(t)
	rec := doPost(h.ShowUsers, `invalid`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUnsuspendUser_InvalidParam(t *testing.T) {
	h, _, _ := newTestHandler(t)
	rec := doPost(h.UnsuspendUser, `{}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Failing repo tests ---

type failingUpdateUserRepo struct {
	*testutil.MockUserRepository
}

func (f *failingUpdateUserRepo) UpdateUser(_ string, _ map[string]any) error { return assert.AnError }

type failingListUsersRepo struct {
	*testutil.MockUserRepository
}

func (f *failingListUsersRepo) ListUsers(_ model.UserListFilter) ([]*model.User, error) {
	return nil, assert.AnError
}

type failingUpdateMetaRepo struct {
	*testutil.MockMetaRepository
}

func (f *failingUpdateMetaRepo) Update(_ map[string]any) error { return assert.AnError }

func TestSuspendUser_UpdateError(t *testing.T) {
	repo := &failingUpdateUserRepo{testutil.NewMockUserRepository()}
	repo.Users["u1"] = &model.User{ID: "u1"}
	metaRepo := testutil.NewMockMetaRepository()
	metaRepo.Meta = &model.Meta{ID: "x"}
	idGen, _ := id.NewGenerator("aidx")
	h := apiadmin.NewHandler(signup.NewService(repo, metaRepo, idGen), metaRepo, repo, idGen)
	rec := doPost(h.SuspendUser, `{"userId":"u1"}`, nil)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestUnsuspendUser_UpdateError(t *testing.T) {
	repo := &failingUpdateUserRepo{testutil.NewMockUserRepository()}
	repo.Users["u1"] = &model.User{ID: "u1"}
	metaRepo := testutil.NewMockMetaRepository()
	metaRepo.Meta = &model.Meta{ID: "x"}
	idGen, _ := id.NewGenerator("aidx")
	h := apiadmin.NewHandler(signup.NewService(repo, metaRepo, idGen), metaRepo, repo, idGen)
	rec := doPost(h.UnsuspendUser, `{"userId":"u1"}`, nil)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestShowUsers_ListError(t *testing.T) {
	repo := &failingListUsersRepo{testutil.NewMockUserRepository()}
	metaRepo := testutil.NewMockMetaRepository()
	metaRepo.Meta = &model.Meta{ID: "x"}
	idGen, _ := id.NewGenerator("aidx")
	h := apiadmin.NewHandler(signup.NewService(repo, metaRepo, idGen), metaRepo, repo, idGen)
	rec := doPost(h.ShowUsers, `{"limit":10}`, nil)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestUpdateMeta_UpdateError(t *testing.T) {
	userRepo := testutil.NewMockUserRepository()
	metaRepo := &failingUpdateMetaRepo{testutil.NewMockMetaRepository()}
	metaRepo.Meta = &model.Meta{ID: "x"}
	idGen, _ := id.NewGenerator("aidx")
	h := apiadmin.NewHandler(signup.NewService(userRepo, metaRepo, idGen), metaRepo, userRepo, idGen)
	rec := doPost(h.UpdateMeta, `{"name":"test"}`, nil)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestAccountsCreate_SignupInternalError(t *testing.T) {
	// User作成で失敗するrepoを使ってINTERNAL_ERRORパスをテスト
	repo := &failingUpdateUserRepo{testutil.NewMockUserRepository()}
	// Create もオーバーライド
	failCreateRepo := &struct {
		*failingUpdateUserRepo
	}{repo}
	metaRepo := testutil.NewMockMetaRepository()
	metaRepo.Meta = &model.Meta{ID: "x"}
	idGen, _ := id.NewGenerator("aidx")
	// signupServiceのuserRepo.Createが失敗するようにする
	failRepo := &failingCreateUserRepoForAdmin{testutil.NewMockUserRepository()}
	h := apiadmin.NewHandler(signup.NewService(failRepo, metaRepo, idGen), metaRepo, failRepo, idGen)
	_ = failCreateRepo // suppress unused
	rec := doPost(h.AccountsCreate, `{"username":"newuser","password":"pass"}`, nil)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

type failingCreateUserRepoForAdmin struct {
	*testutil.MockUserRepository
}

func (f *failingCreateUserRepoForAdmin) Create(_ *model.User) error { return assert.AnError }

func TestUpdateMeta_InvalidJSON(t *testing.T) {
	h, _, _ := newTestHandler(t)
	rec := doPost(h.UpdateMeta, `invalid`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
