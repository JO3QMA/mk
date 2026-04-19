// Package notifications provides /api/notifications/* endpoints.
package notifications

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	"github.com/shiroha-a/mk/internal/core/notification"
	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// Handler handles notifications-related API endpoints.
type Handler struct {
	svc      *notification.Service
	idGen    id.Generator
	userRepo repository.UserRepository
	noteRepo repository.NoteRepository
}

// NewHandler creates a new notifications Handler.
func NewHandler(svc *notification.Service, idGen id.Generator) *Handler {
	return &Handler{svc: svc, idGen: idGen}
}

// SetRepos attaches repositories for resolving user/note objects in notifications.
func (h *Handler) SetRepos(userRepo repository.UserRepository, noteRepo repository.NoteRepository) {
	h.userRepo = userRepo
	h.noteRepo = noteRepo
}

// ListRequest is the request body for notifications.
type ListRequest struct {
	Limit        int      `json:"limit"`
	IncludeTypes []string `json:"includeTypes"`
	ExcludeTypes []string `json:"excludeTypes"`
	SinceID      string   `json:"sinceId"`
	UntilID      string   `json:"untilId"`
}

// Show handles POST /api/i/notifications - returns the authenticated user's
// notification timeline ordered newest first.
func (h *Handler) Show(c echo.Context) error {
	user := middleware.GetUser(c)
	var req ListRequest
	if err := c.Bind(&req); err != nil {
		return apierr.JSONInvalidParam(c)
	}

	rows, err := h.svc.List(c.Request().Context(), user.ID, req.Limit)
	if err != nil {
		return apierr.JSONInternalError(c)
	}

	// includeTypes / excludeTypes フィルタ
	includeSet := make(map[string]bool, len(req.IncludeTypes))
	for _, t := range req.IncludeTypes {
		includeSet[t] = true
	}
	excludeSet := make(map[string]bool, len(req.ExcludeTypes))
	for _, t := range req.ExcludeTypes {
		excludeSet[t] = true
	}

	out := make([]map[string]any, 0, len(rows))
	for _, n := range rows {
		// カーソルベースページネーション
		if req.SinceID != "" && n.ID <= req.SinceID {
			continue
		}
		if req.UntilID != "" && n.ID >= req.UntilID {
			continue
		}
		// タイプフィルタ
		if len(includeSet) > 0 && !includeSet[string(n.Type)] {
			continue
		}
		if excludeSet[string(n.Type)] {
			continue
		}
		// WebSocket 経由の packing と一貫性を保つため entity.PackNotification
		// にdelegateする(Devin #315 指摘)。user/note 取得は handler 側で
		// repo が wire されていれば best-effort。外側の認証ユーザー(`user`
		// at line 48)をshadowしないよう明示的に notifier 名で受ける。
		var notifier *model.User
		if h.userRepo != nil && n.NotifierID != "" {
			if u, err := h.userRepo.FindByID(n.NotifierID); err == nil {
				notifier = u
			}
		}
		var note *model.Note
		if h.noteRepo != nil && n.NoteID != "" {
			if nn, err := h.noteRepo.FindByIDWithUser(n.NoteID); err == nil {
				note = nn
			}
		}
		out = append(out, entity.PackNotification(n, notifier, note, h.idGen))
	}
	return c.JSON(http.StatusOK, out)
}

// MarkAllAsRead handles POST /api/notifications/mark-all-as-read.
func (h *Handler) MarkAllAsRead(c echo.Context) error {
	user := middleware.GetUser(c)
	if err := h.svc.MarkAllAsRead(c.Request().Context(), user.ID); err != nil {
		return apierr.JSONInternalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}

// Create handles POST /api/notifications/create.
// アプリ通知の作成 (簡易版)。
func (h *Handler) Create(c echo.Context) error {
	_ = middleware.GetUser(c)
	return c.NoContent(http.StatusNoContent)
}

// Flush handles POST /api/notifications/flush.
func (h *Handler) Flush(c echo.Context) error {
	user := middleware.GetUser(c)
	if h.svc != nil {
		if err := h.svc.Flush(c.Request().Context(), user.ID); err != nil {
			return apierr.JSONInternalError(c)
		}
	}
	return c.NoContent(http.StatusNoContent)
}

// TestNotification handles POST /api/notifications/test-notification.
func (h *Handler) TestNotification(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}
