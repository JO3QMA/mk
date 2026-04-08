package federation

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	coreinstance "github.com/shiroha-a/mk/internal/core/instance"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newHandler(t *testing.T) (*Handler, *testutil.MockInstanceRepository) {
	t.Helper()
	repo := testutil.NewMockInstanceRepository()
	metaRepo := testutil.NewMockMetaRepository()
	metaRepo.Meta = &model.Meta{}
	idGen, _ := id.NewGenerator("aidx")
	svc := coreinstance.NewService(repo, metaRepo, idGen)
	return NewHandler(svc), repo
}

func newReq(t *testing.T, body string) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func seedInstance(t *testing.T, repo *testutil.MockInstanceRepository, host string) *model.Instance {
	t.Helper()
	inst := &model.Instance{
		ID:               "i_" + host,
		Host:             host,
		FirstRetrievedAt: time.Now(),
		SuspensionState:  model.SuspensionStateNone,
	}
	repo.Instances[host] = inst
	return inst
}

func TestInstances_Empty(t *testing.T) {
	h, _ := newHandler(t)
	c, rec := newReq(t, `{}`)
	require.NoError(t, h.Instances(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestInstances_Filtered(t *testing.T) {
	h, repo := newHandler(t)
	seedInstance(t, repo, "alpha.example")
	seedInstance(t, repo, "beta.example")
	c, rec := newReq(t, `{"host":"alpha"}`)
	require.NoError(t, h.Instances(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "alpha.example")
	assert.NotContains(t, rec.Body.String(), "beta.example")
}

func TestInstances_BadJSON(t *testing.T) {
	h, _ := newHandler(t)
	c, rec := newReq(t, `{not json`)
	require.NoError(t, h.Instances(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// failingInstanceRepo causes List to fail.
type failingInstanceRepo struct {
	*testutil.MockInstanceRepository
}

func (f *failingInstanceRepo) List(_ model.InstanceListFilter) ([]*model.Instance, error) {
	return nil, errors.New("boom")
}

func TestInstances_RepoError(t *testing.T) {
	mock := testutil.NewMockInstanceRepository()
	repo := &failingInstanceRepo{MockInstanceRepository: mock}
	metaRepo := testutil.NewMockMetaRepository()
	metaRepo.Meta = &model.Meta{}
	idGen, _ := id.NewGenerator("aidx")
	svc := coreinstance.NewService(repo, metaRepo, idGen)
	h := NewHandler(svc)
	c, rec := newReq(t, `{}`)
	require.NoError(t, h.Instances(c))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestShowInstance_Success(t *testing.T) {
	h, repo := newHandler(t)
	seedInstance(t, repo, "alpha.example")
	c, rec := newReq(t, `{"host":"alpha.example"}`)
	require.NoError(t, h.ShowInstance(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "alpha.example")
}

func TestShowInstance_BadJSON(t *testing.T) {
	h, _ := newHandler(t)
	c, rec := newReq(t, `{not`)
	require.NoError(t, h.ShowInstance(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestShowInstance_EmptyHost(t *testing.T) {
	h, _ := newHandler(t)
	c, rec := newReq(t, `{}`)
	require.NoError(t, h.ShowInstance(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestShowInstance_NotFound(t *testing.T) {
	h, _ := newHandler(t)
	c, rec := newReq(t, `{"host":"missing.example"}`)
	require.NoError(t, h.ShowInstance(c))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
