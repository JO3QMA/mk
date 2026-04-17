package meta

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/config"
	"github.com/shiroha-a/mk/internal/core/role"
	"github.com/shiroha-a/mk/internal/repository"
)

// Handler handles meta-related API endpoints.
type Handler struct {
	config   *config.Config
	metaRepo repository.MetaRepository
	adRepo   repository.AdRepository
}

// NewHandler creates a new meta Handler.
func NewHandler(cfg *config.Config, metaRepo repository.MetaRepository) *Handler {
	return &Handler{config: cfg, metaRepo: metaRepo}
}

// SetAdRepo wires an AdRepository for the `ads` field on the /api/meta
// response. When unset, ads is returned as an empty array so existing tests
// without ad wiring keep passing.
func (h *Handler) SetAdRepo(r repository.AdRepository) {
	h.adRepo = r
}

// Meta returns server metadata.
// POST /api/meta
// Meta handles POST /api/meta.
// TS互換: detail (boolean, default true)。falseの場合は簡易レスポンスを返す。
func (h *Handler) Meta(c echo.Context) error {
	var params struct {
		Detail *bool `json:"detail"`
	}
	_ = c.Bind(&params)
	detail := params.Detail == nil || *params.Detail

	m, err := h.metaRepo.Fetch()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{
			"error": map[string]any{
				"message": "Internal error.",
				"code":    "INTERNAL_ERROR",
				"id":      "5d37dbcb-891e-41ca-a3d6-e690c97775ac",
			},
		})
	}

	resp := map[string]any{
		"maintainerName":         m.MaintainerName,
		"maintainerEmail":        m.MaintainerEmail,
		"version":                h.config.Version,
		"name":                   m.Name,
		"shortName":              m.ShortName,
		"uri":                    h.config.URL,
		"description":            m.Description,
		"langs":                  m.Langs,
		"disableRegistration":    m.DisableRegistration,
		"emailRequiredForSignup": m.EmailRequiredForSignup,
		"enableHcaptcha":         m.EnableHcaptcha,
		"hcaptchaSiteKey":        m.HcaptchaSiteKey,
		"enableRecaptcha":        m.EnableRecaptcha,
		"recaptchaSiteKey":       m.RecaptchaSiteKey,
		"enableTurnstile":        m.EnableTurnstile,
		"turnstileSiteKey":       m.TurnstileSiteKey,
		"themeColor":             m.ThemeColor,
		"bannerUrl":              m.BannerURL,
		"backgroundImageUrl":     m.BackgroundImageURL,
		"logoImageUrl":           m.LogoImageURL,
		"iconUrl":                m.IconURL,
		"cacheRemoteFiles":       m.CacheRemoteFiles,
		"enableServiceWorker":    m.EnableServiceWorker,
		"swPublickey":            m.SwPublicKey,
		"serverRules":            m.ServerRules,
		"maxNoteTextLength":      3000,

		// フロントエンド互換性フィールド (Phase 4.5c)
		"tosUrl":              m.TermsOfServiceURL,
		"repositoryUrl":       m.RepositoryURL,
		"feedbackUrl":         m.FeedbackURL,
		"impressumUrl":        m.ImpressumURL,
		"privacyPolicyUrl":    m.PrivacyPolicyURL,
		"inquiryUrl":          m.InquiryURL,
		"federation":          m.Federation,
		"defaultLightTheme":   m.DefaultLightTheme,
		"defaultDarkTheme":    m.DefaultDarkTheme,
		"serverErrorImageUrl": m.ServerErrorImageURL,
		"notFoundImageUrl":    m.NotFoundImageURL,
		"infoImageUrl":        m.InfoImageURL,
		"app192IconUrl":       m.App192IconURL,
		"app512IconUrl":       m.App512IconURL,
		// mascotImageUrl: meta 値があればそれを返す。空または nil なら従来の
		// /assets/ai.png にフォールバック (フロントエンドが no-image にならないため)。
		"mascotImageUrl":               mascotURL(m.MascotImageURL),
		"translatorAvailable":          m.DeeplAuthKey != nil && *m.DeeplAuthKey != "",
		"enableEmail":                  m.EnableEmail,
		"enableUrlPreview":             m.URLPreviewEnabled,
		"ads":                          h.serializeActiveAds(),
		"notesPerOneAd":                m.NotesPerOneAd,
		"mediaProxy":                   h.config.MediaProxy,
		"cacheRemoteSensitiveFiles":    m.CacheRemoteSensitiveFiles,
		"requireSetup":                 m.RootUserID == nil,
		"singleUserMode":               m.SingleUserMode,
		"providesTarball":              false,
		"maxFileSize":                  h.config.MaxFileSize,
		"proxyAccountName":             nil,
		"noteSearchableScope":          "local",
		"enableMcaptcha":               m.EnableMcaptcha,
		"mcaptchaSiteKey":              m.McaptchaSiteKey,
		"mcaptchaInstanceUrl":          m.McaptchaInstanceURL,
		"enableTestcaptcha":            m.EnableTestcaptcha,
		"sentryForFrontend":            nil,
		"googleAnalyticsMeasurementId": m.GoogleAnalyticsMeasurementID,
		// clientOptions は jsonb なのでそのまま返す。空 jsonb なら frontend が
		// デフォルト値を使う前提。
		"clientOptions": clientOptionsJSON(m.ClientOptions),

		"policies": role.DefaultPolicies(),

		"features": map[string]any{
			"registration":           !m.DisableRegistration,
			"emailRequiredForSignup": m.EmailRequiredForSignup,
			"hcaptcha":               m.EnableHcaptcha,
			"recaptcha":              m.EnableRecaptcha,
			"turnstile":              m.EnableTurnstile,
			"objectStorage":          m.UseObjectStorage,
			"serviceWorker":          m.EnableServiceWorker,
			"miauth":                 true,
		},
	}

	// detail=false: TS MetaLite互換。features / policies / clientOptions /
	// proxyAccountName / sentryForFrontend のみ省く。登録/captcha/ads等は含める。
	if !detail {
		omit := map[string]struct{}{
			"features":            {},
			"policies":            {},
			"clientOptions":       {},
			"proxyAccountName":    {},
			"sentryForFrontend":   {},
			"noteSearchableScope": {},
			"providesTarball":     {},
			"singleUserMode":      {},
		}
		lite := make(map[string]any, len(resp))
		for k, v := range resp {
			if _, skip := omit[k]; !skip {
				lite[k] = v
			}
		}
		return c.JSON(http.StatusOK, lite)
	}

	return c.JSON(http.StatusOK, resp)
}

// Ping returns a simple pong response.
// POST /api/ping
func (h *Handler) Ping(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{
		"pong": true,
	})
}

// mascotURL applies the legacy fallback for the mascot. Misskey フロントエンドは
// このフィールドが空文字 / 未設定でも特定の no-image を出すだけだが、本家挙動と
// 互換にするため /assets/ai.png をフォールバックに使う。
func mascotURL(v *string) string {
	if v != nil && *v != "" {
		return *v
	}
	return "/assets/ai.png"
}

// clientOptionsJSON returns m.ClientOptions as a generic any. JSON encoder の
// raw bytes をそのまま返すと frontend が parse しないので、空の場合は空 map に
// 正規化する (本家フロントエンドは object を期待する)。
func clientOptionsJSON(raw []byte) any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
}

// serializeActiveAds fetches currently-active ads via adRepo and projects
// them into the Misskey-compatible shape. Returns an empty slice when the
// repo is unwired (tests) or fetch fails, keeping the response stable.
// フロントエンドが notesPerOneAd と組み合わせて timeline に差し込む前提。
func (h *Handler) serializeActiveAds() []map[string]any {
	if h.adRepo == nil {
		return []map[string]any{}
	}
	ads, err := h.adRepo.ListActive(time.Now())
	if err != nil || len(ads) == 0 {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(ads))
	for _, a := range ads {
		out = append(out, map[string]any{
			"id":        a.ID,
			"url":       a.URL,
			"place":     a.Place,
			"ratio":     a.Ratio,
			"imageUrl":  a.ImageURL,
			"dayOfWeek": a.DayOfWeek,
		})
	}
	return out
}
