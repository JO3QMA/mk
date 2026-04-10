package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/config"
	"github.com/shiroha-a/mk/internal/model"
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
		metaJSON := "{}"
		if m, err := metaRepo.Fetch(); err == nil {
			if m.Name != nil {
				instanceName = *m.Name
			}
			metaJSON = buildMetaJSON(cfg, m)
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
<script type="application/json" id="misskey_meta" data-generated-at="%d">%s</script>
<script src="/vite/loader/boot.js"></script>
</head><body>
<noscript><p>Please turn on your JavaScript</p></noscript>
<div id="splash">
<div style="padding:64px;text-align:center">
<p>Loading...</p>
</div>
</div>
</body></html>`, instanceName, cfg.URL, instanceName, viteClientTag,
			cfg.Version, clientEntryJS,
			time.Now().UnixMilli(), metaJSON)

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

// FrontendDistDir returns the path to _frontend_dist_ assets (locales, fonts).
// 環境変数 MISSKEY_FRONTEND_DIST_DIR で上書き可能。
func FrontendDistDir() string {
	if v := os.Getenv("MISSKEY_FRONTEND_DIST_DIR"); v != "" {
		return v
	}
	return filepath.Join("built", "_frontend_dist_")
}

// TwemojiDir returns the path to twemoji SVG files.
// 環境変数 MISSKEY_TWEMOJI_DIR で上書き可能。
func TwemojiDir() string {
	if v := os.Getenv("MISSKEY_TWEMOJI_DIR"); v != "" {
		return v
	}
	return filepath.Join("node_modules", "@discordapp", "twemoji", "dist", "svg")
}

// StaticDir returns the path to static assets (icons, splash, favicon etc.).
// 環境変数 MISSKEY_STATIC_DIR で上書き可能。
func StaticDir() string {
	if v := os.Getenv("MISSKEY_STATIC_DIR"); v != "" {
		return v
	}
	return filepath.Join("assets")
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

// buildMetaJSON constructs the /api/meta equivalent JSON for inline embedding.
// meta handler と同じフィールドを返す。フロントエンドはこのJSONを先に読んで
// /api/meta の呼び出しを省略する。
func buildMetaJSON(cfg *config.Config, m *model.Meta) string {
	resp := map[string]any{
		"maintainerName": m.MaintainerName, "maintainerEmail": m.MaintainerEmail,
		"version": cfg.Version, "name": m.Name, "shortName": m.ShortName,
		"uri": cfg.URL, "description": m.Description, "langs": m.Langs,
		"disableRegistration":    m.DisableRegistration,
		"emailRequiredForSignup": m.EmailRequiredForSignup,
		"enableHcaptcha":         m.EnableHcaptcha, "hcaptchaSiteKey": m.HcaptchaSiteKey,
		"enableRecaptcha": m.EnableRecaptcha, "recaptchaSiteKey": m.RecaptchaSiteKey,
		"enableTurnstile": m.EnableTurnstile, "turnstileSiteKey": m.TurnstileSiteKey,
		"themeColor": m.ThemeColor, "bannerUrl": m.BannerURL,
		"backgroundImageUrl": m.BackgroundImageURL,
		"logoImageUrl":       m.LogoImageURL, "iconUrl": m.IconURL,
		"cacheRemoteFiles":    m.CacheRemoteFiles,
		"enableServiceWorker": m.EnableServiceWorker,
		"swPublickey":         m.SwPublicKey, "serverRules": m.ServerRules,
		"maxNoteTextLength": 3000,
		"tosUrl":            m.TermsOfServiceURL, "repositoryUrl": m.RepositoryURL,
		"feedbackUrl": m.FeedbackURL, "impressumUrl": m.ImpressumURL,
		"privacyPolicyUrl":    m.PrivacyPolicyURL,
		"federation":          m.Federation,
		"translatorAvailable": false, "enableEmail": m.EnableEmail,
		"ads": []any{}, "notesPerOneAd": 0,
		"cacheRemoteSensitiveFiles": m.CacheRemoteSensitiveFiles,
		"requireSetup":              false,
		"features": map[string]any{
			"registration":           !m.DisableRegistration,
			"emailRequiredForSignup": m.EmailRequiredForSignup,
			"hcaptcha":               m.EnableHcaptcha, "recaptcha": m.EnableRecaptcha,
			"turnstile": m.EnableTurnstile, "objectStorage": m.UseObjectStorage,
			"serviceWorker": m.EnableServiceWorker, "miauth": true,
		},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return "{}"
	}
	return string(data)
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
