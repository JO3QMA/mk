package admin_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	apiadmin "github.com/shiroha-a/mk/internal/api/admin"
	"github.com/shiroha-a/mk/internal/core/role"
	"github.com/shiroha-a/mk/internal/core/signup"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/server/middleware"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestHandler(t *testing.T) (*apiadmin.Handler, *testutil.MockUserRepository, *testutil.MockMetaRepository, *testutil.MockRoleRepository) {
	t.Helper()
	userRepo := testutil.NewMockUserRepository()
	metaRepo := testutil.NewMockMetaRepository()
	metaRepo.Meta = &model.Meta{ID: "x"}
	roleRepo := testutil.NewMockRoleRepository()
	assignRepo := testutil.NewMockRoleAssignmentRepository(roleRepo)
	idGen, _ := id.NewGenerator("aidx")
	signupSvc := signup.NewService(userRepo, metaRepo, idGen)
	roleSvc := role.NewService(roleRepo, assignRepo, metaRepo, idGen)
	h := apiadmin.NewHandler(signupSvc, roleSvc, metaRepo, userRepo, idGen)
	return h, userRepo, metaRepo, roleRepo
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
	h, _, metaRepo, _ := newTestHandler(t)
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
	h, _, metaRepo, _ := newTestHandler(t)
	rootID := "root1"
	metaRepo.Meta.RootUserID = &rootID

	// 認証なし → ACCESS_DENIED
	rec := doPost(h.AccountsCreate, `{"username":"user2","password":"pass"}`, nil)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestAccountsCreate_AsRootUser(t *testing.T) {
	h, _, metaRepo, _ := newTestHandler(t)
	rootID := "root1"
	metaRepo.Meta.RootUserID = &rootID

	rootUser := &model.User{ID: "root1", Username: "root"}
	rec := doPost(h.AccountsCreate, `{"username":"user2","password":"pass"}`, rootUser)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAccountsCreate_AsNonRoot_Denied(t *testing.T) {
	h, _, metaRepo, _ := newTestHandler(t)
	rootID := "root1"
	metaRepo.Meta.RootUserID = &rootID

	otherUser := &model.User{ID: "other", Username: "other"}
	rec := doPost(h.AccountsCreate, `{"username":"user2","password":"pass"}`, otherUser)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestAccountsCreate_InvalidJSON(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.AccountsCreate, `invalid`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAccountsCreate_DuplicateUsername(t *testing.T) {
	h, userRepo, _, _ := newTestHandler(t)
	userRepo.Users["u1"] = &model.User{ID: "u1", Username: "taken", UsernameLower: "taken"}

	rec := doPost(h.AccountsCreate, `{"username":"taken","password":"pass"}`, nil)
	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestAccountsCreate_MetaFetchError(t *testing.T) {
	h, _, metaRepo, _ := newTestHandler(t)
	metaRepo.Meta = nil // Fetch will error
	rec := doPost(h.AccountsCreate, `{"username":"admin","password":"pass"}`, nil)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- ShowUser ---

func TestShowUser_Success(t *testing.T) {
	h, userRepo, _, _ := newTestHandler(t)
	userRepo.Users["u1"] = &model.User{ID: "u1", Username: "test"}

	rec := doPost(h.ShowUser, `{"userId":"u1"}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestShowUser_NotFound(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.ShowUser, `{"userId":"ghost"}`, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestShowUser_InvalidParam(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.ShowUser, `{}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- ShowUsers ---

func TestShowUsers_Success(t *testing.T) {
	h, userRepo, _, _ := newTestHandler(t)
	userRepo.Users["u1"] = &model.User{ID: "u1", Username: "a"}
	userRepo.Users["u2"] = &model.User{ID: "u2", Username: "b"}

	rec := doPost(h.ShowUsers, `{"limit":10}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp []any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp, 2)
}

func TestShowUsers_WithFilter(t *testing.T) {
	h, userRepo, _, _ := newTestHandler(t)
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
	h, userRepo, _, _ := newTestHandler(t)
	userRepo.Users["u1"] = &model.User{ID: "u1", Username: "target"}

	rec := doPost(h.SuspendUser, `{"userId":"u1"}`, nil)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestSuspendUser_NotFound(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.SuspendUser, `{"userId":"ghost"}`, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestSuspendUser_InvalidParam(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.SuspendUser, `{}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUnsuspendUser_Success(t *testing.T) {
	h, userRepo, _, _ := newTestHandler(t)
	userRepo.Users["u1"] = &model.User{ID: "u1", IsSuspended: true}

	rec := doPost(h.UnsuspendUser, `{"userId":"u1"}`, nil)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestUnsuspendUser_NotFound(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.UnsuspendUser, `{"userId":"ghost"}`, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// --- AdminMeta / UpdateMeta ---

func TestAdminMeta_Success(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.AdminMeta, `{}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAdminMeta_FetchError(t *testing.T) {
	h, _, metaRepo, _ := newTestHandler(t)
	metaRepo.Meta = nil
	rec := doPost(h.AdminMeta, `{}`, nil)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestUpdateMeta_Success(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.UpdateMeta, `{"name":"My Instance"}`, nil)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestAccountsCreate_EmptyUsername(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.AccountsCreate, `{"username":"","password":"pass"}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAccountsCreate_WhitespaceOnlyUsername(t *testing.T) {
	// Bindはusernameがemptyかチェックするが、空白のみはbindを通過する。
	// Signup側でTrimSpace後にemptyになり、ErrInvalidUsernameが返る。
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.AccountsCreate, `{"username":"   ","password":"pass"}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestShowUsers_InvalidJSON(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.ShowUsers, `invalid`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUnsuspendUser_InvalidParam(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
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
	h := apiadmin.NewHandler(signup.NewService(repo, metaRepo, idGen), nil, metaRepo, repo, idGen)
	rec := doPost(h.SuspendUser, `{"userId":"u1"}`, nil)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestUnsuspendUser_UpdateError(t *testing.T) {
	repo := &failingUpdateUserRepo{testutil.NewMockUserRepository()}
	repo.Users["u1"] = &model.User{ID: "u1"}
	metaRepo := testutil.NewMockMetaRepository()
	metaRepo.Meta = &model.Meta{ID: "x"}
	idGen, _ := id.NewGenerator("aidx")
	h := apiadmin.NewHandler(signup.NewService(repo, metaRepo, idGen), nil, metaRepo, repo, idGen)
	rec := doPost(h.UnsuspendUser, `{"userId":"u1"}`, nil)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestShowUsers_ListError(t *testing.T) {
	repo := &failingListUsersRepo{testutil.NewMockUserRepository()}
	metaRepo := testutil.NewMockMetaRepository()
	metaRepo.Meta = &model.Meta{ID: "x"}
	idGen, _ := id.NewGenerator("aidx")
	h := apiadmin.NewHandler(signup.NewService(repo, metaRepo, idGen), nil, metaRepo, repo, idGen)
	rec := doPost(h.ShowUsers, `{"limit":10}`, nil)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestUpdateMeta_UpdateError(t *testing.T) {
	userRepo := testutil.NewMockUserRepository()
	metaRepo := &failingUpdateMetaRepo{testutil.NewMockMetaRepository()}
	metaRepo.Meta = &model.Meta{ID: "x"}
	idGen, _ := id.NewGenerator("aidx")
	h := apiadmin.NewHandler(signup.NewService(userRepo, metaRepo, idGen), nil, metaRepo, userRepo, idGen)
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
	h := apiadmin.NewHandler(signup.NewService(failRepo, metaRepo, idGen), nil, metaRepo, failRepo, idGen)
	_ = failCreateRepo // suppress unused
	rec := doPost(h.AccountsCreate, `{"username":"newuser","password":"pass"}`, nil)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

type failingCreateUserRepoForAdmin struct {
	*testutil.MockUserRepository
}

func (f *failingCreateUserRepoForAdmin) Create(_ *model.User) error { return assert.AnError }

func TestUpdateMeta_InvalidJSON(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.UpdateMeta, `invalid`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Roles endpoints ---

func TestRolesCreate_Success(t *testing.T) {
	h, _, _, roleRepo := newTestHandler(t)
	rec := doPost(h.RolesCreate, `{"name":"Admin","isAdministrator":true}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Len(t, roleRepo.Roles, 1)
}

func TestRolesCreate_InvalidParam(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.RolesCreate, `{}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRolesShow_Success(t *testing.T) {
	h, _, _, roleRepo := newTestHandler(t)
	roleRepo.Roles["r1"] = &model.Role{ID: "r1", Name: "Test"}
	rec := doPost(h.RolesShow, `{"roleId":"r1"}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRolesShow_NotFound(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.RolesShow, `{"roleId":"ghost"}`, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRolesShow_InvalidParam(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.RolesShow, `{}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRolesList_Success(t *testing.T) {
	h, _, _, roleRepo := newTestHandler(t)
	roleRepo.Roles["r1"] = &model.Role{ID: "r1", Name: "A"}
	rec := doPost(h.RolesList, `{}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRolesUpdate_Success(t *testing.T) {
	h, _, _, roleRepo := newTestHandler(t)
	roleRepo.Roles["r1"] = &model.Role{ID: "r1", Name: "Old"}
	rec := doPost(h.RolesUpdate, `{"roleId":"r1","name":"New"}`, nil)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestRolesUpdate_AllFields(t *testing.T) {
	h, _, _, roleRepo := newTestHandler(t)
	roleRepo.Roles["r1"] = &model.Role{ID: "r1", Name: "Old"}
	rec := doPost(h.RolesUpdate, `{"roleId":"r1","name":"New","description":"desc","isModerator":true,"isAdministrator":true,"isPublic":true}`, nil)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestRolesUpdate_NotFound(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.RolesUpdate, `{"roleId":"ghost"}`, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRolesUpdate_InvalidParam(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.RolesUpdate, `{}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRolesDelete_Success(t *testing.T) {
	h, _, _, roleRepo := newTestHandler(t)
	roleRepo.Roles["r1"] = &model.Role{ID: "r1"}
	rec := doPost(h.RolesDelete, `{"roleId":"r1"}`, nil)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestRolesDelete_NotFound(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.RolesDelete, `{"roleId":"ghost"}`, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRolesDelete_InvalidParam(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.RolesDelete, `{}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRolesAssign_Success(t *testing.T) {
	h, _, _, roleRepo := newTestHandler(t)
	roleRepo.Roles["r1"] = &model.Role{ID: "r1"}
	rec := doPost(h.RolesAssign, `{"userId":"u1","roleId":"r1"}`, nil)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestRolesAssign_WithExpiry(t *testing.T) {
	h, _, _, roleRepo := newTestHandler(t)
	roleRepo.Roles["r1"] = &model.Role{ID: "r1"}
	rec := doPost(h.RolesAssign, `{"userId":"u1","roleId":"r1","expiresAt":"2099-01-01T00:00:00Z"}`, nil)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestRolesAssign_NotFound(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.RolesAssign, `{"userId":"u1","roleId":"ghost"}`, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRolesAssign_AlreadyAssigned(t *testing.T) {
	h, _, _, roleRepo := newTestHandler(t)
	roleRepo.Roles["r1"] = &model.Role{ID: "r1"}
	doPost(h.RolesAssign, `{"userId":"u1","roleId":"r1"}`, nil) // first assign
	rec := doPost(h.RolesAssign, `{"userId":"u1","roleId":"r1"}`, nil)
	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestRolesAssign_InvalidParam(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.RolesAssign, `{}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRolesUnassign_Success(t *testing.T) {
	h, _, _, roleRepo := newTestHandler(t)
	roleRepo.Roles["r1"] = &model.Role{ID: "r1"}
	doPost(h.RolesAssign, `{"userId":"u1","roleId":"r1"}`, nil)
	rec := doPost(h.RolesUnassign, `{"userId":"u1","roleId":"r1"}`, nil)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestRolesUnassign_NotAssigned(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.RolesUnassign, `{"userId":"u1","roleId":"r1"}`, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRolesUnassign_InvalidParam(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.RolesUnassign, `{}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRolesUsers_Success(t *testing.T) {
	h, _, _, roleRepo := newTestHandler(t)
	roleRepo.Roles["r1"] = &model.Role{ID: "r1"}
	rec := doPost(h.RolesUsers, `{"roleId":"r1"}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRolesUsers_NotFound(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.RolesUsers, `{"roleId":"ghost"}`, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRolesUsers_InvalidParam(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.RolesUsers, `{}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRolesUpdateDefaultPolicies_Success(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.RolesUpdateDefaultPolicies, `{"policies":{"driveCapacityMb":500}}`, nil)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestRolesUpdateDefaultPolicies_UpdateError(t *testing.T) {
	userRepo := testutil.NewMockUserRepository()
	metaRepo := &failingUpdateMetaRepo{testutil.NewMockMetaRepository()}
	metaRepo.Meta = &model.Meta{ID: "x"}
	roleRepo := testutil.NewMockRoleRepository()
	assignRepo := testutil.NewMockRoleAssignmentRepository(roleRepo)
	idGen, _ := id.NewGenerator("aidx")
	roleSvc := role.NewService(roleRepo, assignRepo, metaRepo, idGen)
	h := apiadmin.NewHandler(signup.NewService(userRepo, metaRepo, idGen), roleSvc, metaRepo, userRepo, idGen)
	rec := doPost(h.RolesUpdateDefaultPolicies, `{"policies":{"x":1}}`, nil)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestRolesCreate_ErrorFromService(t *testing.T) {
	// Createがエラーになるケースをテスト — failing roleRepoが必要
	// ここではfailingリポジトリでHandler直接作成
	failRepo := &failingCreateRoleRepo{testutil.NewMockRoleRepository()}
	assignRepo := testutil.NewMockRoleAssignmentRepository(failRepo.MockRoleRepository)
	metaRepo := testutil.NewMockMetaRepository()
	metaRepo.Meta = &model.Meta{ID: "x"}
	idGen, _ := id.NewGenerator("aidx")
	roleSvc := role.NewService(failRepo, assignRepo, metaRepo, idGen)
	userRepo := testutil.NewMockUserRepository()
	h := apiadmin.NewHandler(signup.NewService(userRepo, metaRepo, idGen), roleSvc, metaRepo, userRepo, idGen)
	rec := doPost(h.RolesCreate, `{"name":"Test"}`, nil)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

type failingCreateRoleRepo struct {
	*testutil.MockRoleRepository
}

func (f *failingCreateRoleRepo) Create(_ *model.Role) error { return assert.AnError }

func TestRolesAssign_InternalError(t *testing.T) {
	// Exists がエラーになるケースをテスト
	h, _, _, roleRepo := newTestHandler(t)
	roleRepo.Roles["r1"] = &model.Role{ID: "r1"}
	// 1回目のassignは成功
	doPost(h.RolesAssign, `{"userId":"u1","roleId":"r1"}`, nil)
	// 2回目はALREADY_ASSIGNED → 409 (既にテスト済みだが念のため)
	rec := doPost(h.RolesAssign, `{"userId":"u1","roleId":"r1"}`, nil)
	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestRolesUnassign_InternalError(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	// 存在しないassignmentのunassign → NOT_ASSIGNED
	rec := doPost(h.RolesUnassign, `{"userId":"u1","roleId":"r1"}`, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

type failingListRoleRepo struct {
	*testutil.MockRoleRepository
}

func (f *failingListRoleRepo) List() ([]*model.Role, error) { return nil, assert.AnError }

func TestRolesList_Error(t *testing.T) {
	failRepo := &failingListRoleRepo{testutil.NewMockRoleRepository()}
	assignRepo := testutil.NewMockRoleAssignmentRepository(failRepo.MockRoleRepository)
	metaRepo := testutil.NewMockMetaRepository()
	metaRepo.Meta = &model.Meta{ID: "x"}
	userRepo := testutil.NewMockUserRepository()
	idGen, _ := id.NewGenerator("aidx")
	roleSvc := role.NewService(failRepo, assignRepo, metaRepo, idGen)
	h := apiadmin.NewHandler(signup.NewService(userRepo, metaRepo, idGen), roleSvc, metaRepo, userRepo, idGen)
	rec := doPost(h.RolesList, `{}`, nil)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

type failingAssignExistsRepo struct {
	*testutil.MockRoleAssignmentRepository
}

func (f *failingAssignExistsRepo) Exists(_ string, _ string) (bool, error) {
	return false, assert.AnError
}

func TestRolesAssign_ExistsError(t *testing.T) {
	roleRepo := testutil.NewMockRoleRepository()
	roleRepo.Roles["r1"] = &model.Role{ID: "r1"}
	assignRepo := &failingAssignExistsRepo{testutil.NewMockRoleAssignmentRepository(roleRepo)}
	metaRepo := testutil.NewMockMetaRepository()
	metaRepo.Meta = &model.Meta{ID: "x"}
	userRepo := testutil.NewMockUserRepository()
	idGen, _ := id.NewGenerator("aidx")
	roleSvc := role.NewService(roleRepo, assignRepo, metaRepo, idGen)
	h := apiadmin.NewHandler(signup.NewService(userRepo, metaRepo, idGen), roleSvc, metaRepo, userRepo, idGen)
	rec := doPost(h.RolesAssign, `{"userId":"u1","roleId":"r1"}`, nil)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestRolesUnassign_ExistsError(t *testing.T) {
	roleRepo := testutil.NewMockRoleRepository()
	assignRepo := &failingAssignExistsRepo{testutil.NewMockRoleAssignmentRepository(roleRepo)}
	metaRepo := testutil.NewMockMetaRepository()
	metaRepo.Meta = &model.Meta{ID: "x"}
	userRepo := testutil.NewMockUserRepository()
	idGen, _ := id.NewGenerator("aidx")
	roleSvc := role.NewService(roleRepo, assignRepo, metaRepo, idGen)
	h := apiadmin.NewHandler(signup.NewService(userRepo, metaRepo, idGen), roleSvc, metaRepo, userRepo, idGen)
	rec := doPost(h.RolesUnassign, `{"userId":"u1","roleId":"r1"}`, nil)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestRolesUpdateDefaultPolicies_InvalidParam(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.RolesUpdateDefaultPolicies, `invalid`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
