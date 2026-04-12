package i

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/labstack/echo/v4"
	coreemail "github.com/shiroha-a/mk/internal/core/email"
	"github.com/shiroha-a/mk/internal/server/middleware"
	"golang.org/x/crypto/bcrypt"
)

// generateVerifyCode returns a random 16-char hex code for email verification.
func generateVerifyCode() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// verifyURL builds the email verification URL.
func (h *Handler) verifyURL(code string) string {
	base := h.serverURL
	if base == "" {
		base = "https://localhost"
	}
	return base + "/verify-email/" + code
}

// Apps handles POST /api/i/apps.
func (h *Handler) Apps(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

// AuthorizedApps handles POST /api/i/authorized-apps.
func (h *Handler) AuthorizedApps(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

// SigninHistory handles POST /api/i/signin-history.
func (h *Handler) SigninHistory(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

// RevokeToken handles POST /api/i/revoke-token.
func (h *Handler) RevokeToken(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// UpdateEmail handles POST /api/i/update-email.
// 本家 Misskey と同じフロー:
//  1. パスワード検証 (必須)
//  2. email が null → emailRequiredForSignup ならエラー、そうでなければクリア
//  3. email が非null → フォーマット + banned + active/API 検証
//  4. profile の email / emailVerified / emailVerifyCode を更新
//  5. 新 email があれば確認メールを送信
func (h *Handler) UpdateEmail(c echo.Context) error {
	u := middleware.GetUser(c)
	var req struct {
		Password string  `json:"password"`
		Email    *string `json:"email"`
	}
	if err := c.Bind(&req); err != nil {
		return invalidParam(c)
	}

	profile := h.userService.GetProfile(u.ID)
	if profile == nil || profile.Password == nil {
		return internalError(c)
	}

	// パスワード検証
	if err := bcrypt.CompareHashAndPassword([]byte(*profile.Password), []byte(req.Password)); err != nil {
		return c.JSON(http.StatusForbidden, errEnvelope("Incorrect password.", "INCORRECT_PASSWORD", "e86c14a4-0da8-4571-8f36-8a2e9f9b3a00"))
	}

	fields := map[string]any{
		"emailVerified":   false,
		"emailVerifyCode": nil,
	}

	if req.Email == nil {
		// email クリア。emailRequiredForSignup 時はクリア不可。
		if h.metaRepo != nil {
			if m, err := h.metaRepo.Fetch(); err == nil && m.EmailRequiredForSignup {
				return c.JSON(http.StatusBadRequest, errEnvelope("Email is required.", "INVALID_PARAM", "3d81ceae-475f-4600-b2a8-2bc116157532"))
			}
		}
		fields["email"] = nil
	} else {
		addr := *req.Email
		// email validation (banned + format + active/API)
		if h.metaRepo != nil {
			if m, err := h.metaRepo.Fetch(); err == nil {
				svc := coreemail.NewService(m)
				if verr := svc.Validate(c.Request().Context(), addr); verr != nil {
					return c.JSON(http.StatusBadRequest, errEnvelope("Email is not available.", "UNAVAILABLE", "a]504947-b888-4a99-9f62-8c4a0f3a3dab"))
				}
			}
		}
		fields["email"] = addr

		// 確認コード生成 + メール送信
		code := generateVerifyCode()
		fields["emailVerifyCode"] = code

		if h.emailSender != nil {
			go h.emailSender(addr, "Verify your email",
				"Click the link to verify your email:\n"+h.verifyURL(code))
		}
	}

	if err := h.userService.UpdateProfileFields(u.ID, fields); err != nil {
		return internalError(c)
	}

	return h.Me(c)
}

// VerifyEmail handles POST /api/verify-email.
// emailVerifyCode が一致すれば emailVerified を true にする。
func (h *Handler) VerifyEmail(c echo.Context) error {
	var req struct {
		Code string `json:"code"`
	}
	if err := c.Bind(&req); err != nil || req.Code == "" {
		return invalidParam(c)
	}

	profile, err := h.userService.FindProfileByVerifyCode(req.Code)
	if err != nil {
		return c.JSON(http.StatusNotFound, errEnvelope("No such code.", "NO_SUCH_CODE", "1e53842e-b7f4-4e1c-8f1e-8d0a2d9b0c7e"))
	}

	if verr := h.userService.UpdateProfileFields(profile.UserID, map[string]any{
		"emailVerified":   true,
		"emailVerifyCode": nil,
	}); verr != nil {
		return internalError(c)
	}

	return c.NoContent(http.StatusNoContent)
}

// Move handles POST /api/i/move.
func (h *Handler) Move(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// GalleryLikes handles POST /api/i/gallery/likes.
func (h *Handler) GalleryLikes(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

// GalleryPosts handles POST /api/i/gallery/posts.
func (h *Handler) GalleryPosts(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

// PageLikes handles POST /api/i/page-likes.
func (h *Handler) PageLikes(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

// RegistryGetDetail handles POST /api/i/registry/get-detail.
func (h *Handler) RegistryGetDetail(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{})
}

// RegistryKeys handles POST /api/i/registry/keys.
func (h *Handler) RegistryKeys(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

// RegistryScopesWithDomain handles POST /api/i/registry/scopes-with-domain.
func (h *Handler) RegistryScopesWithDomain(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}
