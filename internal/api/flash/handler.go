// Package flash provides /api/flash/* endpoints.
package flash

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	coreflash "github.com/shiroha-a/mk/internal/core/flash"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// Handler handles flash-related API endpoints.
type Handler struct {
	svc *coreflash.Service
}

// NewHandler creates a new flash Handler.
func NewHandler(svc *coreflash.Service) *Handler {
	return &Handler{svc: svc}
}

// CreateRequest is the request body for flash/create.
type CreateRequest struct {
	Title       string   `json:"title"`
	Summary     string   `json:"summary"`
	Script      string   `json:"script"`
	Permissions []string `json:"permissions"`
	Visibility  string   `json:"visibility"`
}

// Create handles POST /api/flash/create.
func (h *Handler) Create(c echo.Context) error {
	user := middleware.GetUser(c)
	var req CreateRequest
	if err := c.Bind(&req); err != nil || req.Title == "" || req.Script == "" {
		return apierr.JSONInvalidParam(c)
	}
	f, err := h.svc.Create(coreflash.CreateInput{
		OwnerID:     user.ID,
		Title:       req.Title,
		Summary:     req.Summary,
		Script:      req.Script,
		Permissions: req.Permissions,
		Visibility:  req.Visibility,
	})
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	return c.JSON(http.StatusOK, flashToMap(f))
}

// ShowRequest is the request body for flash/show.
type ShowRequest struct {
	FlashID string `json:"flashId"`
}

// Show handles POST /api/flash/show.
func (h *Handler) Show(c echo.Context) error {
	user := middleware.GetUser(c)
	var req ShowRequest
	if err := c.Bind(&req); err != nil || req.FlashID == "" {
		return apierr.JSONInvalidParam(c)
	}
	requesterID := ""
	if user != nil {
		requesterID = user.ID
	}
	f, err := h.svc.Show(requesterID, req.FlashID)
	if err != nil {
		return notFound(c)
	}
	return c.JSON(http.StatusOK, flashToMap(f))
}

// UpdateRequest is the request body for flash/update.
type UpdateRequest struct {
	FlashID     string    `json:"flashId"`
	Title       *string   `json:"title"`
	Summary     *string   `json:"summary"`
	Script      *string   `json:"script"`
	Permissions *[]string `json:"permissions"`
	Visibility  *string   `json:"visibility"`
}

// Update handles POST /api/flash/update.
func (h *Handler) Update(c echo.Context) error {
	user := middleware.GetUser(c)
	var req UpdateRequest
	if err := c.Bind(&req); err != nil || req.FlashID == "" {
		return apierr.JSONInvalidParam(c)
	}
	f, err := h.svc.Update(user.ID, req.FlashID, coreflash.UpdateInput{
		Title:       req.Title,
		Summary:     req.Summary,
		Script:      req.Script,
		Permissions: req.Permissions,
		Visibility:  req.Visibility,
	})
	if err != nil {
		switch {
		case errors.Is(err, coreflash.ErrFlashNotFound):
			return notFound(c)
		case errors.Is(err, coreflash.ErrAccessDenied):
			return apierr.JSONAccessDenied(c)
		case errors.Is(err, coreflash.ErrFlashTitleRequired):
			return apierr.JSONInvalidParam(c)
		}
		return apierr.JSONInternalError(c)
	}
	return c.JSON(http.StatusOK, flashToMap(f))
}

// DeleteRequest is the request body for flash/delete.
type DeleteRequest struct {
	FlashID string `json:"flashId"`
}

// Delete handles POST /api/flash/delete.
func (h *Handler) Delete(c echo.Context) error {
	user := middleware.GetUser(c)
	var req DeleteRequest
	if err := c.Bind(&req); err != nil || req.FlashID == "" {
		return apierr.JSONInvalidParam(c)
	}
	if err := h.svc.Delete(user.ID, req.FlashID); err != nil {
		switch {
		case errors.Is(err, coreflash.ErrFlashNotFound):
			return notFound(c)
		case errors.Is(err, coreflash.ErrAccessDenied):
			return apierr.JSONAccessDenied(c)
		}
		return apierr.JSONInternalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}

// PaginationRequest is the shared body for list-style endpoints.
type PaginationRequest struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// SearchRequest is the request body for flash/search.
type SearchRequest struct {
	Query  string `json:"query"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

// My handles POST /api/i/flashs (own list).
func (h *Handler) My(c echo.Context) error {
	user := middleware.GetUser(c)
	var req PaginationRequest
	if err := c.Bind(&req); err != nil {
		return apierr.JSONInvalidParam(c)
	}
	rows, err := h.svc.My(user.ID, req.Limit, req.Offset)
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	return c.JSON(http.StatusOK, flashesToList(rows))
}

// Featured handles POST /api/flash/featured.
func (h *Handler) Featured(c echo.Context) error {
	var req PaginationRequest
	if err := c.Bind(&req); err != nil {
		return apierr.JSONInvalidParam(c)
	}
	rows, err := h.svc.Featured(req.Limit, req.Offset)
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	return c.JSON(http.StatusOK, flashesToList(rows))
}

// Search handles POST /api/flash/search.
func (h *Handler) Search(c echo.Context) error {
	var req SearchRequest
	if err := c.Bind(&req); err != nil || req.Query == "" {
		return apierr.JSONInvalidParam(c)
	}
	rows, err := h.svc.Search(req.Query, req.Limit, req.Offset)
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	return c.JSON(http.StatusOK, flashesToList(rows))
}

// LikeRequest is the request body for flash/like and flash/unlike.
type LikeRequest struct {
	FlashID string `json:"flashId"`
}

// Like handles POST /api/flash/like.
func (h *Handler) Like(c echo.Context) error {
	user := middleware.GetUser(c)
	var req LikeRequest
	if err := c.Bind(&req); err != nil || req.FlashID == "" {
		return apierr.JSONInvalidParam(c)
	}
	if err := h.svc.Like(user.ID, req.FlashID); err != nil {
		switch {
		case errors.Is(err, coreflash.ErrFlashNotFound):
			return notFound(c)
		case errors.Is(err, coreflash.ErrAlreadyLiked):
			return alreadyLiked(c)
		}
		return apierr.JSONInternalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}

// Unlike handles POST /api/flash/unlike.
func (h *Handler) Unlike(c echo.Context) error {
	user := middleware.GetUser(c)
	var req LikeRequest
	if err := c.Bind(&req); err != nil || req.FlashID == "" {
		return apierr.JSONInvalidParam(c)
	}
	if err := h.svc.Unlike(user.ID, req.FlashID); err != nil {
		switch {
		case errors.Is(err, coreflash.ErrFlashNotFound):
			return notFound(c)
		case errors.Is(err, coreflash.ErrNotLiked):
			return notLiked(c)
		}
		return apierr.JSONInternalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}

// MyLikes handles POST /api/i/flashs/likes.
func (h *Handler) MyLikes(c echo.Context) error {
	user := middleware.GetUser(c)
	var req PaginationRequest
	if err := c.Bind(&req); err != nil {
		return apierr.JSONInvalidParam(c)
	}
	rows, err := h.svc.MyLikes(user.ID, req.Limit, req.Offset)
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	return c.JSON(http.StatusOK, flashesToList(rows))
}

func flashesToList(rows []*model.Flash) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, f := range rows {
		out = append(out, flashToMap(f))
	}
	return out
}

func flashToMap(f *model.Flash) map[string]any {
	return map[string]any{
		"id":          f.ID,
		"updatedAt":   f.UpdatedAt,
		"title":       f.Title,
		"summary":     f.Summary,
		"userId":      f.UserID,
		"script":      f.Script,
		"permissions": []string(f.Permissions),
		"likedCount":  f.LikedCount,
		"visibility":  f.Visibility,
	}
}

func notFound(c echo.Context) error {
	return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_FLASH", "No such flash.", "f0d34a1a-d29a-401d-90ba-1982122b5630"))
}

func alreadyLiked(c echo.Context) error {
	return c.JSON(http.StatusBadRequest, apierr.Error("ALREADY_LIKED", "You already liked that flash.", "33106d32-22c2-4cdb-9c2e-29ddf92fd14c"))
}

func notLiked(c echo.Context) error {
	return c.JSON(http.StatusBadRequest, apierr.Error("NOT_LIKED", "You have not liked that flash.", "f5eb37a7-72e4-4c2a-89e1-d56fbafe8b25"))
}
