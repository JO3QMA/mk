package admin

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
)

// CaptchaCurrent handles POST /api/admin/captcha/current.
//
// Returns which captcha provider (if any) is currently enabled and its public
// site key so the admin UI can render the correct configuration form.
func (h *Handler) CaptchaCurrent(c echo.Context) error {
	if h.metaRepo == nil {
		return c.JSON(http.StatusOK, map[string]any{"provider": nil})
	}
	meta, err := h.metaRepo.Fetch()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	var provider any
	var siteKey *string
	switch {
	case meta.EnableHcaptcha:
		provider = "hcaptcha"
		siteKey = meta.HcaptchaSiteKey
	case meta.EnableRecaptcha:
		provider = "recaptcha"
		siteKey = meta.RecaptchaSiteKey
	case meta.EnableTurnstile:
		provider = "turnstile"
		siteKey = meta.TurnstileSiteKey
	}
	return c.JSON(http.StatusOK, map[string]any{
		"provider": provider,
		"siteKey":  siteKey,
	})
}

// CaptchaSave handles POST /api/admin/captcha/save.
func (h *Handler) CaptchaSave(c echo.Context) error {
	var req struct {
		Provider    string  `json:"provider"`
		HcaptchaSK  *string `json:"hcaptchaSiteKey"`
		HcaptchaSS  *string `json:"hcaptchaSecretKey"`
		RecaptchaSK *string `json:"recaptchaSiteKey"`
		RecaptchaSS *string `json:"recaptchaSecretKey"`
		TurnstileSK *string `json:"turnstileSiteKey"`
		TurnstileSS *string `json:"turnstileSecretKey"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "Invalid parameters.", "00000000-0000-0000-0000-000000000000"))
	}
	// provider が空 or "none" の場合は captcha 無効化として扱う。
	switch req.Provider {
	case "", "none", "hcaptcha", "recaptcha", "turnstile":
	default:
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "Unknown captcha provider.", "00000000-0000-0000-0000-000000000000"))
	}
	if h.metaRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	fields := map[string]any{
		"enableHcaptcha":  req.Provider == "hcaptcha",
		"enableRecaptcha": req.Provider == "recaptcha",
		"enableTurnstile": req.Provider == "turnstile",
	}
	if req.HcaptchaSK != nil {
		fields["hcaptchaSiteKey"] = *req.HcaptchaSK
	}
	if req.HcaptchaSS != nil {
		fields["hcaptchaSecretKey"] = *req.HcaptchaSS
	}
	if req.RecaptchaSK != nil {
		fields["recaptchaSiteKey"] = *req.RecaptchaSK
	}
	if req.RecaptchaSS != nil {
		fields["recaptchaSecretKey"] = *req.RecaptchaSS
	}
	if req.TurnstileSK != nil {
		fields["turnstileSiteKey"] = *req.TurnstileSK
	}
	if req.TurnstileSS != nil {
		fields["turnstileSecretKey"] = *req.TurnstileSS
	}
	if err := h.metaRepo.Update(fields); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	return c.NoContent(http.StatusNoContent)
}
