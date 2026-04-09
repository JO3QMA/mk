package roles_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/roles"
	corerole "github.com/shiroha-a/mk/internal/core/role"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestHandler(t *testing.T) (*roles.Handler, *testutil.MockRoleRepository) {
	t.Helper()
	roleRepo := testutil.NewMockRoleRepository()
	assignRepo := testutil.NewMockRoleAssignmentRepository(roleRepo)
	metaRepo := testutil.NewMockMetaRepository()
	metaRepo.Meta = &model.Meta{ID: "x"}
	idGen, _ := id.NewGenerator("aidx")
	svc := corerole.NewService(roleRepo, assignRepo, metaRepo, idGen)
	return roles.NewHandler(svc), roleRepo
}

func doPost(h func(echo.Context) error, body string) *httptest.ResponseRecorder {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	_ = h(c)
	return rec
}

func TestList_PublicOnly(t *testing.T) {
	h, roleRepo := newTestHandler(t)
	roleRepo.Roles["r1"] = &model.Role{ID: "r1", Name: "Public", IsPublic: true}
	roleRepo.Roles["r2"] = &model.Role{ID: "r2", Name: "Private", IsPublic: false}
	rec := doPost(h.List, `{}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	var resp []any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp, 1)
}

func TestList_Empty(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doPost(h.List, `{}`)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestShow_Success(t *testing.T) {
	h, roleRepo := newTestHandler(t)
	roleRepo.Roles["r1"] = &model.Role{ID: "r1", Name: "Pub", IsPublic: true}
	rec := doPost(h.Show, `{"roleId":"r1"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestShow_NotPublic(t *testing.T) {
	h, roleRepo := newTestHandler(t)
	roleRepo.Roles["r1"] = &model.Role{ID: "r1", Name: "Priv", IsPublic: false}
	rec := doPost(h.Show, `{"roleId":"r1"}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestShow_NotFound(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doPost(h.Show, `{"roleId":"ghost"}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestShow_InvalidParam(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doPost(h.Show, `{}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUsers_Success(t *testing.T) {
	h, roleRepo := newTestHandler(t)
	roleRepo.Roles["r1"] = &model.Role{ID: "r1", IsPublic: true}
	rec := doPost(h.Users, `{"roleId":"r1"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestUsers_NotFound(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doPost(h.Users, `{"roleId":"ghost"}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUsers_InvalidParam(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doPost(h.Users, `{}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

type failingListRoleSvc struct{}

func TestList_Error(t *testing.T) {
	roleRepo := &failingListRepo{testutil.NewMockRoleRepository()}
	assignRepo := testutil.NewMockRoleAssignmentRepository(roleRepo.MockRoleRepository)
	metaRepo := testutil.NewMockMetaRepository()
	metaRepo.Meta = &model.Meta{ID: "x"}
	idGen, _ := id.NewGenerator("aidx")
	svc := corerole.NewService(roleRepo, assignRepo, metaRepo, idGen)
	h := roles.NewHandler(svc)
	rec := doPost(h.List, `{}`)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

type failingListRepo struct {
	*testutil.MockRoleRepository
}

func (f *failingListRepo) List() ([]*model.Role, error) { return nil, assert.AnError }
