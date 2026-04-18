package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- RepoAssetsDir ---

func TestRepoAssetsDir_Default(t *testing.T) {
	t.Setenv("MISSKEY_REPO_ASSETS_DIR", "")
	got := RepoAssetsDir()
	assert.Equal(t, filepath.Join("third_party/misskey", "assets"), got)
}

func TestRepoAssetsDir_EnvOverride(t *testing.T) {
	t.Setenv("MISSKEY_REPO_ASSETS_DIR", "/custom/assets")
	got := RepoAssetsDir()
	assert.Equal(t, "/custom/assets", got)
}

// --- Other *Dir functions ---

func TestFrontendDir_Default(t *testing.T) {
	t.Setenv("MISSKEY_FRONTEND_DIR", "")
	got := FrontendDir()
	assert.Equal(t, filepath.Join("third_party/misskey", "built", "_frontend_vite_"), got)
}

func TestFrontendDir_EnvOverride(t *testing.T) {
	t.Setenv("MISSKEY_FRONTEND_DIR", "/custom/frontend")
	assert.Equal(t, "/custom/frontend", FrontendDir())
}

func TestFrontendDistDir_Default(t *testing.T) {
	t.Setenv("MISSKEY_FRONTEND_DIST_DIR", "")
	got := FrontendDistDir()
	assert.Equal(t, filepath.Join("third_party/misskey", "built", "_frontend_dist_"), got)
}

func TestFrontendDistDir_EnvOverride(t *testing.T) {
	t.Setenv("MISSKEY_FRONTEND_DIST_DIR", "/custom/dist")
	assert.Equal(t, "/custom/dist", FrontendDistDir())
}

func TestSwDistDir_Default(t *testing.T) {
	t.Setenv("MISSKEY_SW_DIST_DIR", "")
	t.Setenv("MISSKEY_FRONTEND_DIR", "")
	got := SwDistDir()
	// FrontendDirの親 + "_sw_dist_"
	assert.Equal(t, filepath.Join("third_party/misskey", "built", "_sw_dist_"), got)
}

func TestSwDistDir_EnvOverride(t *testing.T) {
	t.Setenv("MISSKEY_SW_DIST_DIR", "/custom/sw")
	assert.Equal(t, "/custom/sw", SwDistDir())
}

func TestClientAssetsDir_Default(t *testing.T) {
	t.Setenv("MISSKEY_CLIENT_ASSETS_DIR", "")
	got := ClientAssetsDir()
	assert.Equal(t, filepath.Join("third_party/misskey", "packages", "frontend", "assets"), got)
}

func TestClientAssetsDir_EnvOverride(t *testing.T) {
	t.Setenv("MISSKEY_CLIENT_ASSETS_DIR", "/custom/client")
	assert.Equal(t, "/custom/client", ClientAssetsDir())
}

func TestTwemojiDir_Default(t *testing.T) {
	t.Setenv("MISSKEY_TWEMOJI_DIR", "")
	got := TwemojiDir()
	assert.Equal(t, filepath.Join("third_party/misskey", "packages", "backend", "node_modules", "@discordapp", "twemoji", "dist", "svg"), got)
}

func TestTwemojiDir_EnvOverride(t *testing.T) {
	t.Setenv("MISSKEY_TWEMOJI_DIR", "/custom/twemoji")
	assert.Equal(t, "/custom/twemoji", TwemojiDir())
}

func TestStaticDir_Default(t *testing.T) {
	t.Setenv("MISSKEY_STATIC_DIR", "")
	got := StaticDir()
	assert.Equal(t, filepath.Join("third_party/misskey", "packages", "backend", "assets"), got)
}

func TestStaticDir_EnvOverride(t *testing.T) {
	t.Setenv("MISSKEY_STATIC_DIR", "/custom/static")
	assert.Equal(t, "/custom/static", StaticDir())
}

// --- assetsHandler ---

func setupAssetsHandler(t *testing.T) (primary, fallback string) {
	t.Helper()
	primary = t.TempDir()
	fallback = t.TempDir()
	return
}

func doAssetsRequest(handler echo.HandlerFunc, path string) (*httptest.ResponseRecorder, error) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/assets/"+path, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("*")
	c.SetParamValues(path)
	err := handler(c)
	return rec, err
}

func TestAssetsHandler_ServeFromPrimary(t *testing.T) {
	primary, fallback := setupAssetsHandler(t)
	require.NoError(t, os.WriteFile(filepath.Join(primary, "style.css"), []byte("body{}"), 0644))

	h := assetsHandler(primary, fallback)
	rec, err := doAssetsRequest(h, "style.css")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "body{}")
}

func TestAssetsHandler_FallbackToSecondary(t *testing.T) {
	primary, fallback := setupAssetsHandler(t)
	// primaryにはファイルなし、fallbackにのみ配置
	require.NoError(t, os.WriteFile(filepath.Join(fallback, "ai.png"), []byte("PNG"), 0644))

	h := assetsHandler(primary, fallback)
	rec, err := doAssetsRequest(h, "ai.png")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "PNG")
}

func TestAssetsHandler_PrimaryTakesPrecedence(t *testing.T) {
	primary, fallback := setupAssetsHandler(t)
	require.NoError(t, os.WriteFile(filepath.Join(primary, "dup.txt"), []byte("primary"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(fallback, "dup.txt"), []byte("fallback"), 0644))

	h := assetsHandler(primary, fallback)
	rec, err := doAssetsRequest(h, "dup.txt")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "primary")
}

func TestAssetsHandler_NotFoundInEither(t *testing.T) {
	primary, fallback := setupAssetsHandler(t)

	h := assetsHandler(primary, fallback)
	_, err := doAssetsRequest(h, "missing.txt")
	assert.Equal(t, echo.ErrNotFound, err)
}

func TestAssetsHandler_DirectoryIgnored(t *testing.T) {
	primary, fallback := setupAssetsHandler(t)
	// ディレクトリと同名のパスはファイルとして配信しない
	require.NoError(t, os.MkdirAll(filepath.Join(primary, "subdir"), 0755))

	h := assetsHandler(primary, fallback)
	_, err := doAssetsRequest(h, "subdir")
	assert.Equal(t, echo.ErrNotFound, err)
}

func TestAssetsHandler_PathTraversalBlocked(t *testing.T) {
	primary, fallback := setupAssetsHandler(t)
	// filepath.Clean("/"+name) でトラバーサルを正規化するため親ディレクトリには到達しない
	h := assetsHandler(primary, fallback)
	_, err := doAssetsRequest(h, "../../../etc/passwd")
	assert.Equal(t, echo.ErrNotFound, err)
}

// --- detectClientEntry ---

func TestDetectClientEntry_NoManifest(t *testing.T) {
	t.Setenv("MISSKEY_FRONTEND_DIR", t.TempDir())
	info := detectClientEntry()
	assert.Empty(t, info.Script)
	assert.Nil(t, info.CSS)
}

func TestDetectClientEntry_ValidManifest(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MISSKEY_FRONTEND_DIR", dir)
	manifest := `{"src/_boot_.ts":{"file":"assets/boot.abc.js","isEntry":true,"css":["assets/style.def.css"]}}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0644))

	info := detectClientEntry()
	assert.Equal(t, "assets/boot.abc.js", info.Script)
	assert.Equal(t, []string{"assets/style.def.css"}, info.CSS)
}

func TestDetectClientEntry_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MISSKEY_FRONTEND_DIR", dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.json"), []byte("{invalid"), 0644))

	info := detectClientEntry()
	assert.Empty(t, info.Script)
}

func TestDetectClientEntry_NoBootEntry(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MISSKEY_FRONTEND_DIR", dir)
	manifest := `{"src/other.ts":{"file":"other.js","isEntry":true}}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0644))

	info := detectClientEntry()
	assert.Empty(t, info.Script)
}

// --- clientOptionsJSON ---

func TestClientOptionsJSON_Empty(t *testing.T) {
	result := clientOptionsJSON(nil)
	assert.Equal(t, map[string]any{}, result)
}

func TestClientOptionsJSON_EmptySlice(t *testing.T) {
	result := clientOptionsJSON([]byte{})
	assert.Equal(t, map[string]any{}, result)
}

func TestClientOptionsJSON_Valid(t *testing.T) {
	result := clientOptionsJSON([]byte(`{"key":"val"}`))
	m, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "val", m["key"])
}

func TestClientOptionsJSON_Invalid(t *testing.T) {
	result := clientOptionsJSON([]byte(`not json`))
	assert.Equal(t, map[string]any{}, result)
}

// --- deepMergeManifest ---

func TestDeepMergeManifest_Overwrite(t *testing.T) {
	dst := map[string]any{"name": "old"}
	src := map[string]any{"name": "new"}
	deepMergeManifest(dst, src)
	assert.Equal(t, "new", dst["name"])
}

func TestDeepMergeManifest_AddNew(t *testing.T) {
	dst := map[string]any{"a": 1}
	src := map[string]any{"b": 2}
	deepMergeManifest(dst, src)
	assert.Equal(t, 1, dst["a"])
	assert.Equal(t, 2, dst["b"])
}

func TestDeepMergeManifest_NestedMerge(t *testing.T) {
	dst := map[string]any{
		"params": map[string]any{"title": "t", "text": "x"},
	}
	src := map[string]any{
		"params": map[string]any{"text": "updated", "url": "u"},
	}
	deepMergeManifest(dst, src)
	params := dst["params"].(map[string]any)
	assert.Equal(t, "t", params["title"])
	assert.Equal(t, "updated", params["text"])
	assert.Equal(t, "u", params["url"])
}

func TestDeepMergeManifest_MapOverwrittenByScalar(t *testing.T) {
	dst := map[string]any{
		"params": map[string]any{"title": "t"},
	}
	src := map[string]any{
		"params": "string_value",
	}
	deepMergeManifest(dst, src)
	assert.Equal(t, "string_value", dst["params"])
}
