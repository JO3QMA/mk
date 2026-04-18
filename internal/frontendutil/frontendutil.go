package frontendutil

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v4"
)

const frontendBase = "third_party/misskey"

// FrontendDir returns the path to built frontend assets.
// 環境変数 MISSKEY_FRONTEND_DIR で上書き可能。
func FrontendDir() string {
	if v := os.Getenv("MISSKEY_FRONTEND_DIR"); v != "" {
		return v
	}
	return filepath.Join(frontendBase, "built", "_frontend_vite_")
}

// FrontendDistDir returns the path to _frontend_dist_ assets (locales, fonts).
// 環境変数 MISSKEY_FRONTEND_DIST_DIR で上書き可能。
func FrontendDistDir() string {
	if v := os.Getenv("MISSKEY_FRONTEND_DIST_DIR"); v != "" {
		return v
	}
	return filepath.Join(frontendBase, "built", "_frontend_dist_")
}

// SwDistDir returns the path to _sw_dist_ assets (service worker sw.js).
// 環境変数 MISSKEY_SW_DIST_DIR で上書き可能。Frontend build の
// sibling ディレクトリに出力されるのでデフォルトは FrontendDir の親 +
// "_sw_dist_"。
func SwDistDir() string {
	if v := os.Getenv("MISSKEY_SW_DIST_DIR"); v != "" {
		return v
	}
	return filepath.Join(filepath.Dir(FrontendDir()), "_sw_dist_")
}

// ClientAssetsDir returns the path to frontend client assets (game images, etc.).
// 環境変数 MISSKEY_CLIENT_ASSETS_DIR で上書き可能。
func ClientAssetsDir() string {
	if v := os.Getenv("MISSKEY_CLIENT_ASSETS_DIR"); v != "" {
		return v
	}
	return filepath.Join(frontendBase, "packages", "frontend", "assets")
}

// TwemojiDir returns the path to twemoji SVG files.
// 環境変数 MISSKEY_TWEMOJI_DIR で上書き可能。pnpm の hoisted node_modules は
// packages/backend 配下に配置されるため、そちらを参照する。
func TwemojiDir() string {
	if v := os.Getenv("MISSKEY_TWEMOJI_DIR"); v != "" {
		return v
	}
	return filepath.Join(frontendBase, "packages", "backend", "node_modules", "@discordapp", "twemoji", "dist", "svg")
}

// StaticDir returns the path to static assets (icons, splash, favicon etc.).
// 環境変数 MISSKEY_STATIC_DIR で上書き可能。本家 TS では packages/backend/assets
// が /static-assets として配信される。
func StaticDir() string {
	if v := os.Getenv("MISSKEY_STATIC_DIR"); v != "" {
		return v
	}
	return filepath.Join(frontendBase, "packages", "backend", "assets")
}

// RepoAssetsDir returns the path to the top-level repository assets directory
// (ai.png, banner images, etc.).
// 環境変数 MISSKEY_REPO_ASSETS_DIR で上書き可能。
func RepoAssetsDir() string {
	if v := os.Getenv("MISSKEY_REPO_ASSETS_DIR"); v != "" {
		return v
	}
	return filepath.Join(frontendBase, "assets")
}

// ClientEntryInfo holds the Vite entry point script and its CSS dependencies.
type ClientEntryInfo struct {
	Script string
	CSS    []string
}

// DetectClientEntry reads the Vite manifest to find the entry script and CSS.
// ビルド済みアセットが存在しない場合は空値を返す (dev mode)。
func DetectClientEntry() ClientEntryInfo {
	manifestPath := filepath.Join(FrontendDir(), "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return ClientEntryInfo{}
	}
	var manifest map[string]struct {
		File    string   `json:"file"`
		IsEntry bool     `json:"isEntry"`
		CSS     []string `json:"css"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return ClientEntryInfo{}
	}
	if entry, ok := manifest["src/_boot_.ts"]; ok {
		return ClientEntryInfo{Script: entry.File, CSS: entry.CSS}
	}
	return ClientEntryInfo{}
}

// AssetsHandler returns a handler that tries to serve files from primary dir
// first, then falls back to fallback dir. This avoids Echo's limitation of
// only supporting one handler per route pattern.
func AssetsHandler(primary, fallback string) echo.HandlerFunc {
	return func(c echo.Context) error {
		name := c.Param("*")
		// primaryディレクトリから探す
		fp := filepath.Join(primary, filepath.Clean("/"+name))
		if info, err := os.Stat(fp); err == nil && !info.IsDir() {
			return c.File(fp)
		}
		// fallbackディレクトリから探す
		fp = filepath.Join(fallback, filepath.Clean("/"+name))
		if info, err := os.Stat(fp); err == nil && !info.IsDir() {
			return c.File(fp)
		}
		return echo.ErrNotFound
	}
}
