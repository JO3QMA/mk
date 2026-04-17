// Package blocking provides /api/blocking/* endpoints.
package blocking

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	coreblocking "github.com/shiroha-a/mk/internal/core/blocking"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// Handler handles blocking-related API endpoints.
type Handler struct {
	svc *coreblocking.Service
}

// NewHandler creates a new blocking Handler.
func NewHandler(svc *coreblocking.Service) *Handler {
	return &Handler{svc: svc}
}

// PairRequest is the request body for blocking/create and blocking/delete.
type PairRequest struct {
	UserID string `json:"userId"`
}

// Create handles POST /api/blocking/create.
func (h *Handler) Create(c echo.Context) error {
	user := middleware.GetUser(c)
	var req PairRequest
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return apierr.JSONInvalidParam(c)
	}

	if _, err := h.svc.Block(user.ID, req.UserID); err != nil {
		switch {
		case errors.Is(err, coreblocking.ErrSelfBlock):
			return c.JSON(http.StatusBadRequest, apierr.Error("BLOCKEE_IS_YOURSELF", "You cannot block yourself.", "88b19138-f28d-42c0-8499-6a31bbd0fdc6"))
		case errors.Is(err, coreblocking.ErrBlockeeNotFound):
			return apierr.JSONNoSuchUser(c)
		case errors.Is(err, coreblocking.ErrAlreadyBlocking):
			return c.JSON(http.StatusBadRequest, apierr.Error("ALREADY_BLOCKING", "You are already blocking that user.", "787fed64-acb9-464a-82eb-afbd745b9614"))
		}
		return apierr.JSONInternalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}

// Delete handles POST /api/blocking/delete.
func (h *Handler) Delete(c echo.Context) error {
	user := middleware.GetUser(c)
	var req PairRequest
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return apierr.JSONInvalidParam(c)
	}
	if err := h.svc.Unblock(user.ID, req.UserID); err != nil {
		switch {
		case errors.Is(err, coreblocking.ErrSelfBlock):
			return c.JSON(http.StatusBadRequest, apierr.Error("BLOCKEE_IS_YOURSELF", "You cannot unblock yourself.", "06f6fac6-524b-473c-a354-e97a40ae6eac"))
		case errors.Is(err, coreblocking.ErrNotBlocking):
			return c.JSON(http.StatusBadRequest, apierr.Error("NOT_BLOCKING", "You are not blocking that user.", "291b2efa-60c6-45c0-9f6a-045c8f9b02cd"))
		}
		return apierr.JSONInternalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}

// ListRequest is the request body for blocking/list.
type ListRequest struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// List handles POST /api/blocking/list.
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
	for _, b := range rows {
		out = append(out, map[string]any{
			"id":        b.ID,
			"blockeeId": b.BlockeeID,
		})
	}
	return c.JSON(http.StatusOK, out)
}
