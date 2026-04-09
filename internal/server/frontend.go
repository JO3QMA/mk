package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/config"
	"github.com/shiroha-a/mk/internal/repository"
)

// frontendHTML generates the HTML shell for the Misskey frontend.
// ビルド済みアセットがある場合は CLIENT_ENTRY を設定して production モードで配信。
// なければ Vite dev server 経由の development モードで配信。
func frontendHTML(cfg *config.Config, metaRepo repository.MetaRepository) echo.HandlerFunc {
	// ビルド済みアセットからCLIENT_ENTRYを取得
	clientEntry := detectClientEntry()

	return func(c echo.Context) error {
		instanceName := "Misskey"
		if m, err := metaRepo.Fetch(); err == nil && m.Name != nil {
			instanceName = *m.Name
		}

		// CLIENT_ENTRYの設定
		clientEntryJS := "null"
		viteClientTag := `<script type="module" src="/vite/@vite/client"></script>`
		if clientEntry != "" {
			clientEntryJS = fmt.Sprintf("'%s'", clientEntry)
			viteClientTag = "" // production ではVite clientは不要
		}

		html := fmt.Sprintf(`<!DOCTYPE html>
<html><head>
<meta charset="UTF-8">
<meta name="application-name" content="Misskey">
<meta name="referer" content="origin">
<meta property="og:site_name" content="%s">
<meta property="instance_url" content="%s">
<meta name="viewport" content="width=device-width, initial-scale=1, minimum-scale=1, maximum-scale=1, user-scalable=no, viewport-fit=cover">
<title>%s</title>
%s
<link rel="stylesheet" href="/vite/loader/style.css">
<script>const VERSION = '%s'; const CLIENT_ENTRY = %s; const LANGS = ["ja-JP","en-US"];</script>
<script type="application/json" id="misskey_meta" data-generated-at="0">{}</script>
<script src="/vite/loader/boot.js"></script>
</head><body>
<noscript><p>Please turn on your JavaScript</p></noscript>
<div id="splash">
<div style="padding:64px;text-align:center">
<p>Loading...</p>
</div>
</div>
</body></html>`, instanceName, cfg.URL, instanceName, viteClientTag, cfg.Version, clientEntryJS)

		return c.HTML(http.StatusOK, html)
	}
}

// FrontendDir returns the path to built frontend assets.
// 環境変数 MISSKEY_FRONTEND_DIR で上書き可能。デフォルトは built/_frontend_vite_。
func FrontendDir() string {
	if v := os.Getenv("MISSKEY_FRONTEND_DIR"); v != "" {
		return v
	}
	return filepath.Join("built", "_frontend_vite_")
}

// detectClientEntry reads the Vite manifest to find the entry script path.
// ビルド済みアセットが存在しない場合は空文字列を返す (dev mode)。
func detectClientEntry() string {
	manifestPath := filepath.Join(FrontendDir(), "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return ""
	}
	var manifest map[string]struct {
		File    string `json:"file"`
		IsEntry bool   `json:"isEntry"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return ""
	}
	if entry, ok := manifest["src/_boot_.ts"]; ok {
		return entry.File
	}
	return ""
}

// newViteProxy creates a reverse proxy handler that forwards requests to
// the Vite dev server. フロントエンドの開発サーバーへのプロキシ。
func newViteProxy(target string) echo.HandlerFunc {
	remote, err := url.Parse(target)
	if err != nil {
		panic("invalid vite proxy target: " + target)
	}
	proxy := httputil.NewSingleHostReverseProxy(remote)
	origDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		origDirector(req)
		req.Host = remote.Host
	}

	return func(c echo.Context) error {
		proxy.ServeHTTP(c.Response(), c.Request())
		return nil
	}
}
