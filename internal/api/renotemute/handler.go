// Package renotemute provides /api/renote-mute/* endpoints.
package renotemute

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	coremuting "github.com/shiroha-a/mk/internal/core/muting"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// Handler handles renote-mute endpoints.
type Handler struct {
	svc *coremuting.RenoteService
}

// NewHandler creates a new Handler.
func NewHandler(svc *coremuting.RenoteService) *Handler {
	return &Handler{svc: svc}
}

// PairRequest is the body for create/delete.
type PairRequest struct {
	UserID string `json:"userId"`
}

// Create handles POST /api/renote-mute/create.
func (h *Handler) Create(c echo.Context) error {
	user := middleware.GetUser(c)
	var req PairRequest
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return apierr.JSONInvalidParam(c)
	}
	if _, err := h.svc.Mute(user.ID, req.UserID); err != nil {
		switch {
		case errors.Is(err, coremuting.ErrSelfMute):
			return c.JSON(http.StatusBadRequest, apierr.Error("MUTEE_IS_YOURSELF", "You cannot mute yourself.", "37285718-52f7-4aef-b7de-c38b8e8a8420"))
		case errors.Is(err, coremuting.ErrMuteeNotFound):
			return apierr.JSONNoSuchUser(c)
		case errors.Is(err, coremuting.ErrAlreadyMuting):
			return c.JSON(http.StatusBadRequest, apierr.Error("ALREADY_MUTING", "You are already muting that user.", "ccfecbe4-1f1c-4fc2-8bdb-9f3672ab7191"))
		}
		return apierr.JSONInternalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}

// Delete handles POST /api/renote-mute/delete.
func (h *Handler) Delete(c echo.Context) error {
	user := middleware.GetUser(c)
	var req PairRequest
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return apierr.JSONInvalidParam(c)
	}
	if err := h.svc.Unmute(user.ID, req.UserID); err != nil {
		switch {
		case errors.Is(err, coremuting.ErrSelfMute):
			return c.JSON(http.StatusBadRequest, apierr.Error("MUTEE_IS_YOURSELF", "You cannot unmute yourself.", "afa929cc-95f0-4e30-92c1-b4b5e5e0f38e"))
		case errors.Is(err, coremuting.ErrNotMuting):
			return c.JSON(http.StatusBadRequest, apierr.Error("NOT_MUTING", "You are not muting that user.", "5467d020-daa9-4553-81e1-135c0c35a96d"))
		}
		return apierr.JSONInternalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}

// ListRequest is the body for renote-mute/list.
type ListRequest struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// List handles POST /api/renote-mute/list.
func (h *Handler) List(c echo.Context) error {
	user := middleware.GetUser(c)
	var req ListRequest
	if err := c.Bind(&req); err != nil {
		return apierr.JSONInvalidParam(c)
	}
	if req.Limit <= 0 {
		req.Limit = 30
	}
	if req.Limit > 100 {
		req.Limit = 100
	}
	rows, err := h.svc.List(user.ID, req.Limit, req.Offset)
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	out := make([]map[string]any, 0, len(rows))
	for _, m := range rows {
		out = append(out, map[string]any{
			"id":      m.ID,
			"muteeId": m.MuteeID,
		})
	}
	return c.JSON(http.StatusOK, out)
}
