package meta

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/config"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestHandler() (*Handler, *testutil.MockMetaRepository) {
	cfg := &config.Config{
		Version: "2026.3.2",
		URL:     "https://misskey.example.com",
	}
	metaRepo := testutil.NewMockMetaRepository()
	h := NewHandler(cfg, metaRepo)
	return h, metaRepo
}

func TestMeta(t *testing.T) {
	h, metaRepo := newTestHandler()

	name := "Test Instance"
	desc := "A test instance"
	maintainerName := "admin"
	metaRepo.Meta = &model.Meta{
		ID:             "x",
		Name:           &name,
		Description:    &desc,
		MaintainerName: &maintainerName,
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/meta", nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Meta(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	assert.Equal(t, "Test Instance", resp["name"])
	assert.Equal(t, "A test instance", resp["description"])
	assert.Equal(t, "admin", resp["maintainerName"])
	assert.Equal(t, "2026.3.2", resp["version"])
	assert.Equal(t, "https://misskey.example.com", resp["uri"])
	assert.Equal(t, float64(3000), resp["maxNoteTextLength"])

	features, ok := resp["features"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, features["miauth"])
}

func TestMeta_NoMeta(t *testing.T) {
	h, _ := newTestHandler()
	// metaRepo.Metaはnil（未設定）

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/meta", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Meta(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestPing(t *testing.T) {
	h, _ := newTestHandler()

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/ping", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Ping(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["pong"])
}
