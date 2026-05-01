package admin

import (
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	"github.com/shiroha-a/mk/internal/queue"
)

// AccountsDelete handles POST /api/admin/accounts/delete.
func (h *Handler) AccountsDelete(c echo.Context) error {
	var req struct {
		UserID string `json:"userId"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return c.NoContent(http.StatusNoContent)
	}
	_ = h.userRepo.UpdateUser(req.UserID, map[string]any{"isSuspended": true, "isDeleted": true})
	h.scheduleAccountCascade(req.UserID)
	return c.NoContent(http.StatusNoContent)
}

// AccountsFindByEmail handles POST /api/admin/accounts/find-by-email.
// user_profile.email 列を検索して、紐づく user を返す。本家 Misskey の
// admin/accounts/find-by-email と同等。
func (h *Handler) AccountsFindByEmail(c echo.Context) error {
	var req struct {
		Email string `json:"email"`
	}
	if err := c.Bind(&req); err != nil || req.Email == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "email is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	profile, err := h.userRepo.FindProfileByEmail(req.Email)
	if err != nil {
		return c.JSON(http.StatusNotFound, apierr.Error("USER_NOT_FOUND", "User not found.", "a504947-b888-4a99-9f62-8c4a0f3a3dab"))
	}
	user, err := h.userRepo.FindByID(profile.UserID)
	if err != nil {
		return c.JSON(http.StatusNotFound, apierr.Error("USER_NOT_FOUND", "User not found.", "a504947-b888-4a99-9f62-8c4a0f3a3dab"))
	}
	// 他の admin エンドポイント (ShowUser 等) と同じ packAdminUser を通して
	// Misskey 本家互換のレスポンス整形をする。生 model.User を返すと
	// inbox / sharedInbox / usernameLower 等の内部フィールドが漏れ、
	// createdAt / roles / policies 等のフロントが期待するフィールドが欠落する。
	return c.JSON(http.StatusOK, h.packAdminUser(user, profile))
}

// DeleteAccount handles POST /api/admin/delete-account. AccountsDelete と
// 機能的には同じだが、本家 Misskey が両 endpoint を持つので互換性のため
// 別 handler として保持する。
func (h *Handler) DeleteAccount(c echo.Context) error {
	var req struct {
		UserID string `json:"userId"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return c.NoContent(http.StatusNoContent)
	}
	_ = h.userRepo.UpdateUser(req.UserID, map[string]any{"isSuspended": true, "isDeleted": true})
	h.scheduleAccountCascade(req.UserID)
	return c.NoContent(http.StatusNoContent)
}

// scheduleAccountCascade queues the background cascade deletion. Errors
// from the enqueuer are logged but never surfaced — the admin flag flip
// is the user-visible source of truth, so a failed enqueue only delays
// the cleanup until the next manual retry.
func (h *Handler) scheduleAccountCascade(userID string) {
	if h.deleteAccountEnqueuer == nil || userID == "" {
		return
	}
	if err := h.deleteAccountEnqueuer.EnqueueDeleteAccount(queue.DeleteAccountPayload{UserID: userID}); err != nil {
		slog.Warn("admin: enqueue delete-account failed", "userId", userID, "err", err)
	}
}
