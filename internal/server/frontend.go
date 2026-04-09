package server

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/config"
	"github.com/shiroha-a/mk/internal/repository"
)

// frontendHTML generates the minimal HTML shell for the Misskey frontend.
// Misskey TS の base.tsx に相当する。フロントエンドの bootloader が
// このHTML内の meta タグとグローバル変数を読み取って起動する。
func frontendHTML(cfg *config.Config, metaRepo repository.MetaRepository) echo.HandlerFunc {
	return func(c echo.Context) error {
		instanceName := "Misskey"
		if m, err := metaRepo.Fetch(); err == nil && m.Name != nil {
			instanceName = *m.Name
		}

		html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="application-name" content="Misskey">
<meta property="instance_url" content="%s">
<meta property="og:site_name" content="%s">
<title>%s</title>
<script>
var VERSION = "%s";
var CLIENT_ENTRY = null;
var LANGS = [["ja-JP","\u65e5\u672c\u8a9e"],["en-US","English"]];
</script>
<link rel="stylesheet" href="/vite/loader/style.css">
</head>
<body>
<div id="splash">
<div style="padding:64px;text-align:center">
<svg width="64" height="64" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
<circle cx="12" cy="12" r="10"/>
<line x1="12" y1="8" x2="12" y2="12"/>
<line x1="12" y1="16" x2="12" y2="16"/>
</svg>
<p>Loading...</p>
</div>
</div>
<script src="/vite/loader/boot.js"></script>
</body>
</html>`, cfg.URL, instanceName, instanceName, cfg.Version)

		return c.HTML(http.StatusOK, html)
	}
}

// newViteProxy creates a reverse proxy handler that forwards requests to
// the Vite dev server. フロントエンドの開発サーバーへのプロキシ。
func newViteProxy(target string) echo.HandlerFunc {
	remote, err := url.Parse(target)
	if err != nil {
		panic("invalid vite proxy target: " + target)
	}
	proxy := httputil.NewSingleHostReverseProxy(remote)

	return func(c echo.Context) error {
		c.Request().URL.Host = remote.Host
		c.Request().URL.Scheme = remote.Scheme
		c.Request().Header.Set("X-Forwarded-Host", c.Request().Host)
		proxy.ServeHTTP(c.Response(), c.Request())
		return nil
	}
}
