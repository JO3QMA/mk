package i

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/core/twofactor"
	"github.com/shiroha-a/mk/internal/server/middleware"
	"golang.org/x/crypto/bcrypt"
)

// TwoFARegister handles POST /api/i/2fa/register.
// TOTP秘密鍵を生成してtempSecretに保存、QRコードURLを返す。
func (h *Handler) TwoFARegister(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		Password string `json:"password"`
	}
	if err := c.Bind(&req); err != nil || req.Password == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "password is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}
	profile := h.userService.GetProfile(user.ID)
	if profile == nil || profile.Password == nil {
		return c.JSON(http.StatusForbidden, apiError("ACCESS_DENIED", "No password set.", "1fb7cb09-d46a-4fff-b8df-057708cce513"))
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*profile.Password), []byte(req.Password)); err != nil {
		return c.JSON(http.StatusForbidden, apiError("INCORRECT_PASSWORD", "Incorrect password.", "932c904e-9460-45b7-9ce6-7ed33be7eb2c"))
	}

	secret, uri, err := twofactor.GenerateSecret("Misskey", user.Username)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apiError("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}

	// tempSecretに保存 (doneで確認後にsecretに移動)
	_ = h.userService.UpdateProfileFields(user.ID, map[string]any{"twoFactorTempSecret": secret})

	return c.JSON(http.StatusOK, map[string]any{
		"qr":     uri,
		"url":    uri,
		"secret": secret,
		"label":  user.Username,
		"issuer": "Misskey",
	})
}

// TwoFADone handles POST /api/i/2fa/done.
// TOTPコードを検証し、2FAを有効化する。
func (h *Handler) TwoFADone(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		Token string `json:"token"`
	}
	if err := c.Bind(&req); err != nil || req.Token == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "token is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}

	profile := h.userService.GetProfile(user.ID)
	if profile == nil || profile.TwoFactorTempSecret == nil {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "2FA registration not started.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}

	if !twofactor.Validate(req.Token, *profile.TwoFactorTempSecret) {
		return c.JSON(http.StatusForbidden, apiError("INVALID_TOKEN", "Invalid token.", "00000000-0000-0000-0000-000000000000"))
	}

	// tempSecretをsecretに移動、2FAを有効化
	_ = h.userService.UpdateProfileFields(user.ID, map[string]any{
		"twoFactorSecret":     *profile.TwoFactorTempSecret,
		"twoFactorTempSecret": nil,
		"twoFactorEnabled":    true,
	})

	return c.NoContent(http.StatusNoContent)
}

// TwoFAUnregister handles POST /api/i/2fa/unregister.
// 2FAを無効化する。
func (h *Handler) TwoFAUnregister(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		Password string `json:"password"`
	}
	if err := c.Bind(&req); err != nil || req.Password == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "password is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}
	profile := h.userService.GetProfile(user.ID)
	if profile == nil || profile.Password == nil {
		return c.JSON(http.StatusForbidden, apiError("ACCESS_DENIED", "No password set.", "1fb7cb09-d46a-4fff-b8df-057708cce513"))
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*profile.Password), []byte(req.Password)); err != nil {
		return c.JSON(http.StatusForbidden, apiError("INCORRECT_PASSWORD", "Incorrect password.", "932c904e-9460-45b7-9ce6-7ed33be7eb2c"))
	}

	_ = h.userService.UpdateProfileFields(user.ID, map[string]any{
		"twoFactorSecret":  nil,
		"twoFactorEnabled": false,
	})

	return c.NoContent(http.StatusNoContent)
}

// TwoFARegisterKey handles POST /api/i/2fa/register-key — WebAuthn (将来対応).
func (h *Handler) TwoFARegisterKey(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// TwoFAKeyDone handles POST /api/i/2fa/key-done — WebAuthn (将来対応).
func (h *Handler) TwoFAKeyDone(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// TwoFARemoveKey handles POST /api/i/2fa/remove-key — WebAuthn (将来対応).
func (h *Handler) TwoFARemoveKey(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// TwoFAUpdateKey handles POST /api/i/2fa/update-key — WebAuthn (将来対応).
func (h *Handler) TwoFAUpdateKey(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// TwoFAPasswordLess handles POST /api/i/2fa/password-less — WebAuthn (将来対応).
func (h *Handler) TwoFAPasswordLess(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}
