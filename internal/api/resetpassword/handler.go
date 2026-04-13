package resetpassword

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/misc"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

// EmailSender sends an email (to, subject, body).
type EmailSender func(to, subject, body string)

// Handler handles password reset endpoints.
type Handler struct {
	userRepo  repository.UserRepository
	resetRepo repository.PasswordResetRequestRepository
	idGen     id.Generator
	email     EmailSender
	serverURL string
}

// NewHandler creates a new password reset Handler.
func NewHandler(userRepo repository.UserRepository, resetRepo repository.PasswordResetRequestRepository, idGen id.Generator) *Handler {
	return &Handler{userRepo: userRepo, resetRepo: resetRepo, idGen: idGen}
}

// SetEmailSender attaches an EmailSender for sending reset emails.
func (h *Handler) SetEmailSender(s EmailSender) { h.email = s }

// SetServerURL sets the base URL for reset links.
func (h *Handler) SetServerURL(u string) { h.serverURL = u }

func apiError(code, message, errID string) map[string]any {
	return map[string]any{
		"error": map[string]any{"message": message, "code": code, "id": errID},
	}
}

// RequestReset handles POST /api/request-reset-password.
// ユーザーが見つからない/email不一致/未認証でも成功レスポンスを返す（情報漏洩防止）。
func (h *Handler) RequestReset(c echo.Context) error {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
	}
	if err := c.Bind(&req); err != nil || req.Username == "" || req.Email == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "username and email are required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}

	// ユーザー検索 (ローカルのみ)
	user, err := h.userRepo.FindByUsernameLower(req.Username, nil)
	if err != nil {
		return c.NoContent(http.StatusNoContent)
	}

	// profile取得 → email照合 + emailVerified確認
	profile, err := h.userRepo.FindProfileByUserID(user.ID)
	if err != nil || profile.Email == nil || *profile.Email != req.Email || !profile.EmailVerified {
		return c.NoContent(http.StatusNoContent)
	}

	// 64文字のランダムトークン生成
	token := misc.SecureRandomHex(64)
	now := time.Now()
	resetReq := &model.PasswordResetRequest{
		ID:     h.idGen.Generate(now),
		Token:  token,
		UserID: user.ID,
	}
	if err := h.resetRepo.Create(resetReq); err != nil {
		return c.NoContent(http.StatusNoContent)
	}

	// リセットメール送信
	if h.email != nil {
		link := fmt.Sprintf("%s/reset-password/%s", h.serverURL, token)
		go h.email(*profile.Email, "Password reset",
			fmt.Sprintf("Use the following link to reset your password:\n%s", link))
	}

	return c.NoContent(http.StatusNoContent)
}

// Reset handles POST /api/reset-password.
func (h *Handler) Reset(c echo.Context) error {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := c.Bind(&req); err != nil || req.Token == "" || req.Password == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "token and password are required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}

	// bcryptは72バイトまでしか受け付けない
	if len(req.Password) > 72 {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "Password too long.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}

	resetReq, err := h.resetRepo.FindByToken(req.Token)
	if err != nil {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "Invalid token.", "6382759e-0a0d-4e32-893e-0e1e66cec4d5"))
	}

	// 30分の有効期限チェック (IDからタイムスタンプを算出)
	issuedAt, err := h.idGen.ParseTime(resetReq.ID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "Invalid token.", "6382759e-0a0d-4e32-893e-0e1e66cec4d5"))
	}

	// Misskey本家: 30分経過で期限切れ
	if time.Since(issuedAt) > 30*time.Minute {
		_ = h.resetRepo.Delete(resetReq.ID)
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "Token expired.", "6382759e-0a0d-4e32-893e-0e1e66cec4d5"))
	}

	// bcrypt hash (cost 8、Misskey本家と同じ)
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), 8)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apiError("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}

	// パスワード更新
	pw := string(hashed)
	if err := h.userRepo.UpdateProfile(resetReq.UserID, map[string]any{"password": pw}); err != nil {
		return c.JSON(http.StatusInternalServerError, apiError("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}

	// トークン削除
	_ = h.resetRepo.Delete(resetReq.ID)

	return c.NoContent(http.StatusNoContent)
}
