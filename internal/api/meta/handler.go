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
		"maxNoteTextLength":      3000,

		// フロントエンド互換性フィールド (Phase 4.5c)
		"tosUrl":                       m.TermsOfServiceURL,
		"repositoryUrl":                m.RepositoryURL,
		"feedbackUrl":                  m.FeedbackURL,
		"impressumUrl":                 m.ImpressumURL,
		"privacyPolicyUrl":             m.PrivacyPolicyURL,
		"inquiryUrl":                   nil,
		"federation":                   m.Federation,
		"defaultLightTheme":            nil,
		"defaultDarkTheme":             nil,
		"serverErrorImageUrl":          nil,
		"notFoundImageUrl":             nil,
		"infoImageUrl":                 nil,
		"mascotImageUrl":               "/assets/ai.png",
		"translatorAvailable":          false,
		"enableEmail":                  m.EnableEmail,
		"enableUrlPreview":             false,
		"ads":                          []any{},
		"notesPerOneAd":                0,
		"mediaProxy":                   "",
		"cacheRemoteSensitiveFiles":    m.CacheRemoteSensitiveFiles,
		"requireSetup":                 false,
		"providesTarball":              false,
		"maxFileSize":                  h.config.MaxFileSize,
		"proxyAccountName":             nil,
		"noteSearchableScope":          "local",
		"enableMcaptcha":               false,
		"mcaptchaSiteKey":              nil,
		"mcaptchaInstanceUrl":          nil,
		"enableTestcaptcha":            false,
		"sentryForFrontend":            nil,
		"googleAnalyticsMeasurementId": nil,
		"clientOptions":                map[string]any{},

		"policies": defaultPolicies(),

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

	return c.JSON(http.StatusOK, resp)
}

// defaultPolicies returns the Misskey default policies matching the TS implementation.
func defaultPolicies() map[string]any {
	return map[string]any{
		"gtlAvailable":               true,
		"ltlAvailable":               true,
		"canPublicNote":              true,
		"mentionLimit":               20,
		"canInvite":                  false,
		"inviteLimit":                0,
		"inviteLimitCycle":           10080,
		"inviteExpirationTime":       0,
		"canManageCustomEmojis":      false,
		"canManageAvatarDecorations": false,
		"canSearchNotes":             false,
		"canSearchUsers":             true,
		"canUseTranslator":           true,
		"canHideAds":                 false,
		"driveCapacityMb":            100,
		"maxFileSizeMb":              30,
		"alwaysMarkNsfw":             false,
		"canUpdateBioMedia":          true,
		"pinLimit":                   5,
		"antennaLimit":               5,
		"wordMuteLimit":              200,
		"webhookLimit":               3,
		"clipLimit":                  10,
		"noteEachClipsLimit":         200,
		"userListLimit":              10,
		"userEachUserListsLimit":     50,
		"rateLimitFactor":            1,
		"avatarDecorationLimit":      1,
		"canImportAntennas":          false,
		"canImportBlocking":          false,
		"canImportFollowing":         false,
		"canImportMuting":            false,
		"canImportUserLists":         false,
		"chatAvailability":           "available",
		"uploadableFileTypes":        []string{"text/*", "application/json", "image/*", "video/*", "audio/*"},
		"noteDraftLimit":             10,
		"scheduledNoteLimit":         1,
		"watermarkAvailable":         true,
	}
}

// Ping returns a simple pong response.
// POST /api/ping
func (h *Handler) Ping(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{
		"pong": true,
	})
}
