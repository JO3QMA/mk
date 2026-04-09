// Package notifications provides /api/notifications/* endpoints.
package notifications

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/core/notification"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// Handler handles notifications-related API endpoints.
type Handler struct {
	svc   *notification.Service
	idGen id.Generator
}

// NewHandler creates a new notifications Handler.
func NewHandler(svc *notification.Service, idGen id.Generator) *Handler {
	return &Handler{svc: svc, idGen: idGen}
}

// ListRequest is the request body for notifications.
type ListRequest struct {
	Limit int `json:"limit"`
}

// Show handles POST /api/i/notifications - returns the authenticated user's
// notification timeline ordered newest first.
func (h *Handler) Show(c echo.Context) error {
	user := middleware.GetUser(c)
	var req ListRequest
	if err := c.Bind(&req); err != nil {
		return invalidParam(c)
	}

	rows, err := h.svc.List(c.Request().Context(), user.ID, req.Limit)
	if err != nil {
		return internalError(c)
	}

	out := make([]map[string]any, 0, len(rows))
	for _, n := range rows {
		entry := map[string]any{
			"id":        n.ID,
			"createdAt": n.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
			"type":      string(n.Type),
		}
		if n.NotifierID != "" {
			entry["userId"] = n.NotifierID
		}
		if n.NoteID != "" {
			entry["noteId"] = n.NoteID
		}
		if n.Reaction != "" {
			entry["reaction"] = n.Reaction
		}
		out = append(out, entry)
	}
	return c.JSON(http.StatusOK, out)
}

// MarkAllAsRead handles POST /api/notifications/mark-all-as-read.
func (h *Handler) MarkAllAsRead(c echo.Context) error {
	user := middleware.GetUser(c)
	if err := h.svc.MarkAllAsRead(c.Request().Context(), user.ID); err != nil {
		return internalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}

func invalidParam(c echo.Context) error {
	return c.JSON(http.StatusBadRequest, map[string]any{
		"error": map[string]any{
			"message": "Invalid param.",
			"code":    "INVALID_PARAM",
			"id":      "3d81ceae-475f-4600-b2a8-2bc116157532",
		},
	})
}

func internalError(c echo.Context) error {
	return c.JSON(http.StatusInternalServerError, map[string]any{
		"error": map[string]any{
			"message": "Internal error.",
			"code":    "INTERNAL_ERROR",
			"id":      "5d37dbcb-891e-41ca-a3d6-e690c97775ac",
		},
	})
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
		_ = h.svc.MarkAllAsRead(c.Request().Context(), user.ID)
	}
	return c.NoContent(http.StatusNoContent)
}

// TestNotification handles POST /api/notifications/test-notification.
func (h *Handler) TestNotification(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}
