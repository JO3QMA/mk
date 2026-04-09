package meta

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/config"
	"github.com/shiroha-a/mk/internal/repository"
)

// Handler handles meta-related API endpoints.
type Handler struct {
	config   *config.Config
	metaRepo repository.MetaRepository
}

// NewHandler creates a new meta Handler.
func NewHandler(cfg *config.Config, metaRepo repository.MetaRepository) *Handler {
	return &Handler{config: cfg, metaRepo: metaRepo}
}

// Meta returns server metadata.
// POST /api/meta
func (h *Handler) Meta(c echo.Context) error {
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
		"policies":               m.Policies,
		"maxNoteTextLength":      3000,

		// フロントエンド互換性フィールド (Phase 4.5i)
		"tosUrl":                    m.TermsOfServiceURL,
		"repositoryUrl":             m.RepositoryURL,
		"feedbackUrl":               m.FeedbackURL,
		"impressumUrl":              m.ImpressumURL,
		"privacyPolicyUrl":          m.PrivacyPolicyURL,
		"federation":                m.Federation,
		"defaultLightTheme":         nil,
		"defaultDarkTheme":          nil,
		"serverErrorImageUrl":       nil,
		"notFoundImageUrl":          nil,
		"infoImageUrl":              nil,
		"app512IconUrl":             nil,
		"translatorAvailable":       false,
		"enableEmail":               m.EnableEmail,
		"enableUrlPreview":          false,
		"ads":                       []any{},
		"notesPerOneAd":             0,
		"mediaProxy":                "",
		"cacheRemoteSensitiveFiles": m.CacheRemoteSensitiveFiles,
		"requireSetup":              false,

		"features": map[string]any{
			"registration":           !m.DisableRegistration,
			"emailRequiredForSignup": m.EmailRequiredForSignup,
			"hcaptcha":               m.EnableHcaptcha,
			"recaptcha":              m.EnableRecaptcha,
			"turnstile":              m.EnableTurnstile,
			"objectStorage":          m.UseObjectStorage,
			"serviceWorker":          m.EnableServiceWorker,
			"miauth":                 true,
			"localTimeline":          true,
			"globalTimeline":         true,
		},
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
