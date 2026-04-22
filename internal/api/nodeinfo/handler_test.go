package nodeinfo

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

func TestVersion2_1(t *testing.T) {
	h := NewHandler(&config.Config{Version: "0.0.0", Host: "example.com"})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/nodeinfo/2.1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	require.NoError(t, h.Version2_1(c))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "2.1", resp["version"])
	sw := resp["software"].(map[string]any)
	assert.Equal(t, "misskey-go", sw["name"])
	assert.Contains(t, sw["version"], "compatible: misskey 0.0.0")
}

func TestNodeinfoVersion(t *testing.T) {
	assert.Equal(t, "0.0.1 (compatible: misskey 2026.3.2)", nodeinfoVersion("0.0.1", "2026.3.2"))
}

// admin で設定した Meta.Name / Description がnodeinfo.metadata に反映される
// (#348)。これが効かないとリモートが受け取る nodeName はconfig default
// (= host) のまま更新されない。
func TestVersion2_1_ReflectsMetaName(t *testing.T) {
	metaRepo := testutil.NewMockMetaRepository()
	name := "My Custom Instance"
	desc := "hello world"
	mn := "admin"
	me := "admin@example.com"
	metaRepo.Meta = &model.Meta{
		ID: "x", Name: &name, Description: &desc,
		MaintainerName: &mn, MaintainerEmail: &me,
	}
	h := NewHandler(&config.Config{Version: "0.0.0", Host: "fallback.example"})
	h.SetMetaRepo(metaRepo)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/nodeinfo/2.1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	require.NoError(t, h.Version2_1(c))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	meta := resp["metadata"].(map[string]any)
	assert.Equal(t, "My Custom Instance", meta["nodeName"])
	assert.Equal(t, "hello world", meta["nodeDescription"])
	maint := meta["maintainer"].(map[string]any)
	assert.Equal(t, "admin", maint["name"])
	assert.Equal(t, "admin@example.com", maint["email"])
}

// Meta未設定時は cfg.Host が nodeName に入る。
func TestVersion2_1_FallbackToConfigHost(t *testing.T) {
	h := NewHandler(&config.Config{Version: "0.0.0", Host: "example.com"})
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/nodeinfo/2.1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	require.NoError(t, h.Version2_1(c))

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	meta := resp["metadata"].(map[string]any)
	assert.Equal(t, "example.com", meta["nodeName"])
}
