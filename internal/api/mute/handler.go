// Package mute provides /api/mute/* endpoints.
package mute

import (
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	coremuting "github.com/shiroha-a/mk/internal/core/muting"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// Handler handles mute-related API endpoints.
type Handler struct {
	svc *coremuting.Service
}

// NewHandler creates a new mute Handler.
func NewHandler(svc *coremuting.Service) *Handler {
	return &Handler{svc: svc}
}

// CreateRequest is the request body for mute/create.
type CreateRequest struct {
	UserID    string `json:"userId"`
	ExpiresAt *int64 `json:"expiresAt"`
}

// Create handles POST /api/mute/create.
func (h *Handler) Create(c echo.Context) error {
	user := middleware.GetUser(c)
	var req CreateRequest
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return invalidParam(c)
	}

	var expires *time.Time
	if req.ExpiresAt != nil {
		t := time.UnixMilli(*req.ExpiresAt)
		expires = &t
	}

	if _, err := h.svc.Mute(user.ID, req.UserID, expires); err != nil {
		switch {
		case errors.Is(err, coremuting.ErrSelfMute):
			return c.JSON(http.StatusBadRequest, map[string]any{
				"error": map[string]any{
					"message": "You cannot mute yourself.",
					"code":    "MUTEE_IS_YOURSELF",
					"id":      "a4619cb2-5f23-484b-9301-94c903074e10",
				},
			})
		case errors.Is(err, coremuting.ErrMuteeNotFound):
			return noSuchUser(c)
		case errors.Is(err, coremuting.ErrAlreadyMuting):
			return c.JSON(http.StatusBadRequest, map[string]any{
				"error": map[string]any{
					"message": "You are already muting that user.",
					"code":    "ALREADY_MUTING",
					"id":      "7e7359cb-160c-4956-b08f-4d1c653cd007",
				},
			})
		}
		return internalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}

// PairRequest is the request body for mute/delete.
type PairRequest struct {
	UserID string `json:"userId"`
}

// Delete handles POST /api/mute/delete.
func (h *Handler) Delete(c echo.Context) error {
	user := middleware.GetUser(c)
	var req PairRequest
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return invalidParam(c)
	}
	if err := h.svc.Unmute(user.ID, req.UserID); err != nil {
		switch {
		case errors.Is(err, coremuting.ErrSelfMute):
			return c.JSON(http.StatusBadRequest, map[string]any{
				"error": map[string]any{
					"message": "You cannot unmute yourself.",
					"code":    "MUTEE_IS_YOURSELF",
					"id":      "f428b029-6b39-4d48-a1d2-cc1ae6dd5cf9",
				},
			})
		case errors.Is(err, coremuting.ErrNotMuting):
			return c.JSON(http.StatusBadRequest, map[string]any{
				"error": map[string]any{
					"message": "You are not muting that user.",
					"code":    "NOT_MUTING",
					"id":      "5467d020-daa9-4553-81e1-135c0c35a96d",
				},
			})
		}
		return internalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}

// ListRequest is the request body for mute/list.
type ListRequest struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// List handles POST /api/mute/list.
func (h *Handler) List(c echo.Context) error {
	user := middleware.GetUser(c)
	var req ListRequest
	if err := c.Bind(&req); err != nil {
		return invalidParam(c)
	}
	if req.Limit <= 0 {
		req.Limit = 30
	}
	if req.Limit > 100 {
		req.Limit = 100
	}
	rows, err := h.svc.List(user.ID, req.Limit, req.Offset)
	if err != nil {
		return internalError(c)
	}
	out := make([]map[string]any, 0, len(rows))
	for _, m := range rows {
		entry := map[string]any{
			"id":      m.ID,
			"muteeId": m.MuteeID,
		}
		if m.ExpiresAt != nil {
			entry["expiresAt"] = m.ExpiresAt.UnixMilli()
		}
		out = append(out, entry)
	}
	return c.JSON(http.StatusOK, out)
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

func noSuchUser(c echo.Context) error {
	return c.JSON(http.StatusNotFound, map[string]any{
		"error": map[string]any{
			"message": "No such user.",
			"code":    "NO_SUCH_USER",
			"id":      "4362f8dc-731f-4ad8-a694-be5a88922a24",
		},
	})
}
