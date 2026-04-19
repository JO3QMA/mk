// Package pages provides /api/pages/* endpoints.
package pages

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	corepage "github.com/shiroha-a/mk/internal/core/page"
	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// Handler handles page-related API endpoints.
type Handler struct {
	svc   *corepage.Service
	idGen id.Generator
}

// NewHandler creates a new pages Handler. idGen is required so that
// responses include createdAt derived from the aidx-encoded page ID.
func NewHandler(svc *corepage.Service, idGen id.Generator) *Handler {
	return &Handler{svc: svc, idGen: idGen}
}

// CreateRequest is the request body for pages/create. Content / Variables
// are stored verbatim into jsonb columns; we don't interpret them.
type CreateRequest struct {
	Title               string               `json:"title"`
	Name                string               `json:"name"`
	Summary             *string              `json:"summary"`
	Content             json.RawMessage      `json:"content"`
	Variables           json.RawMessage      `json:"variables"`
	Script              string               `json:"script"`
	EyeCatchingImageID  *string              `json:"eyeCatchingImageId"`
	Font                string               `json:"font"`
	AlignCenter         bool                 `json:"alignCenter"`
	HideTitleWhenPinned bool                 `json:"hideTitleWhenPinned"`
	Visibility          model.PageVisibility `json:"visibility"`
}

// Create handles POST /api/pages/create.
func (h *Handler) Create(c echo.Context) error {
	user := middleware.GetUser(c)
	var req CreateRequest
	if err := c.Bind(&req); err != nil || req.Title == "" || req.Name == "" {
		return apierr.JSONInvalidParam(c)
	}
	p, err := h.svc.Create(corepage.CreateInput{
		OwnerID:             user.ID,
		Title:               req.Title,
		Name:                req.Name,
		Summary:             req.Summary,
		AlignCenter:         req.AlignCenter,
		HideTitleWhenPinned: req.HideTitleWhenPinned,
		Font:                req.Font,
		EyeCatchingImageID:  req.EyeCatchingImageID,
		Content:             req.Content,
		Variables:           req.Variables,
		Script:              req.Script,
		Visibility:          req.Visibility,
	})
	if err != nil {
		if errors.Is(err, corepage.ErrPageNameConflict) {
			return nameConflict(c)
		}
		return apierr.JSONInternalError(c)
	}
	return c.JSON(http.StatusOK, h.pageToMap(p))
}

// ShowRequest is the request body for pages/show. pageId か (userId, name) の
// どちらかを指定する。
type ShowRequest struct {
	PageID string `json:"pageId"`
	UserID string `json:"userId"`
	Name   string `json:"name"`
}

// Show handles POST /api/pages/show.
func (h *Handler) Show(c echo.Context) error {
	user := middleware.GetUser(c)
	var req ShowRequest
	if err := c.Bind(&req); err != nil {
		return apierr.JSONInvalidParam(c)
	}
	requesterID := ""
	if user != nil {
		requesterID = user.ID
	}
	var (
		p   *model.Page
		err error
	)
	switch {
	case req.PageID != "":
		p, err = h.svc.Show(requesterID, req.PageID)
	case req.UserID != "" && req.Name != "":
		p, err = h.svc.ShowByName(requesterID, req.UserID, req.Name)
	default:
		return apierr.JSONInvalidParam(c)
	}
	if err != nil {
		if errors.Is(err, corepage.ErrAccessDenied) {
			return apierr.JSONAccessDenied(c)
		}
		return notFound(c)
	}
	return c.JSON(http.StatusOK, h.pageToMap(p))
}

// UpdateRequest is the request body for pages/update.
type UpdateRequest struct {
	PageID              string                `json:"pageId"`
	Title               *string               `json:"title"`
	Name                *string               `json:"name"`
	Summary             *string               `json:"summary"`
	Content             json.RawMessage       `json:"content"`
	Variables           json.RawMessage       `json:"variables"`
	Script              *string               `json:"script"`
	EyeCatchingImageID  *string               `json:"eyeCatchingImageId"`
	Font                *string               `json:"font"`
	AlignCenter         *bool                 `json:"alignCenter"`
	HideTitleWhenPinned *bool                 `json:"hideTitleWhenPinned"`
	Visibility          *model.PageVisibility `json:"visibility"`
}

// Update handles POST /api/pages/update.
func (h *Handler) Update(c echo.Context) error {
	user := middleware.GetUser(c)
	var req UpdateRequest
	if err := c.Bind(&req); err != nil || req.PageID == "" {
		return apierr.JSONInvalidParam(c)
	}
	in := corepage.UpdateInput{
		Title:               req.Title,
		Name:                req.Name,
		AlignCenter:         req.AlignCenter,
		HideTitleWhenPinned: req.HideTitleWhenPinned,
		Font:                req.Font,
		Script:              req.Script,
		Visibility:          req.Visibility,
	}
	if req.Summary != nil {
		s := req.Summary
		in.Summary = &s
	}
	if req.EyeCatchingImageID != nil {
		s := req.EyeCatchingImageID
		in.EyeCatchingImageID = &s
	}
	if req.Content != nil {
		in.Content = req.Content
	}
	if req.Variables != nil {
		in.Variables = req.Variables
	}
	p, err := h.svc.Update(user.ID, req.PageID, in)
	if err != nil {
		switch {
		case errors.Is(err, corepage.ErrPageNotFound):
			return notFound(c)
		case errors.Is(err, corepage.ErrAccessDenied):
			return apierr.JSONAccessDenied(c)
		case errors.Is(err, corepage.ErrPageNameRequired),
			errors.Is(err, corepage.ErrPageTitleRequired):
			return apierr.JSONInvalidParam(c)
		case errors.Is(err, corepage.ErrPageNameConflict):
			return nameConflict(c)
		}
		return apierr.JSONInternalError(c)
	}
	return c.JSON(http.StatusOK, h.pageToMap(p))
}

// DeleteRequest is the request body for pages/delete.
type DeleteRequest struct {
	PageID string `json:"pageId"`
}

// Delete handles POST /api/pages/delete.
func (h *Handler) Delete(c echo.Context) error {
	user := middleware.GetUser(c)
	var req DeleteRequest
	if err := c.Bind(&req); err != nil || req.PageID == "" {
		return apierr.JSONInvalidParam(c)
	}
	if err := h.svc.Delete(user.ID, req.PageID); err != nil {
		switch {
		case errors.Is(err, corepage.ErrPageNotFound):
			return notFound(c)
		case errors.Is(err, corepage.ErrAccessDenied):
			return apierr.JSONAccessDenied(c)
		}
		return apierr.JSONInternalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}

// MyRequest is the request body for i/pages (own pages list).
type MyRequest struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// My handles POST /api/i/pages.
func (h *Handler) My(c echo.Context) error {
	user := middleware.GetUser(c)
	var req MyRequest
	if err := c.Bind(&req); err != nil {
		return apierr.JSONInvalidParam(c)
	}
	rows, err := h.svc.ListByUser(user.ID, req.Limit, req.Offset)
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	return c.JSON(http.StatusOK, h.pagesToList(rows))
}

// FeaturedRequest is the request body for pages/featured.
type FeaturedRequest struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// Featured handles POST /api/pages/featured.
func (h *Handler) Featured(c echo.Context) error {
	var req FeaturedRequest
	if err := c.Bind(&req); err != nil {
		return apierr.JSONInvalidParam(c)
	}
	rows, err := h.svc.Featured(req.Limit, req.Offset)
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	return c.JSON(http.StatusOK, h.pagesToList(rows))
}

// LikeRequest is the request body for pages/like and pages/unlike.
type LikeRequest struct {
	PageID string `json:"pageId"`
}

// Like handles POST /api/pages/like.
func (h *Handler) Like(c echo.Context) error {
	user := middleware.GetUser(c)
	var req LikeRequest
	if err := c.Bind(&req); err != nil || req.PageID == "" {
		return apierr.JSONInvalidParam(c)
	}
	if err := h.svc.Like(user.ID, req.PageID); err != nil {
		switch {
		case errors.Is(err, corepage.ErrPageNotFound):
			return notFound(c)
		case errors.Is(err, corepage.ErrAccessDenied):
			return apierr.JSONAccessDenied(c)
		case errors.Is(err, corepage.ErrAlreadyLiked):
			return alreadyLiked(c)
		}
		return apierr.JSONInternalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}

// Unlike handles POST /api/pages/unlike.
func (h *Handler) Unlike(c echo.Context) error {
	user := middleware.GetUser(c)
	var req LikeRequest
	if err := c.Bind(&req); err != nil || req.PageID == "" {
		return apierr.JSONInvalidParam(c)
	}
	if err := h.svc.Unlike(user.ID, req.PageID); err != nil {
		switch {
		case errors.Is(err, corepage.ErrPageNotFound):
			return notFound(c)
		case errors.Is(err, corepage.ErrNotLiked):
			return notLiked(c)
		}
		return apierr.JSONInternalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}

// pagesToList packs a slice of pages using h.idGen so every element gets a
// consistent createdAt alongside the other fields.
func (h *Handler) pagesToList(rows []*model.Page) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, p := range rows {
		out = append(out, h.pageToMap(p))
	}
	return out
}

// pageToMap delegates to entity.PackPage so the JSON shape (timestamp
// format, field set) stays in sync between /api/pages/* and other places
// that embed a page (e.g. pinnedPage on /api/i and /api/users/show).
func (h *Handler) pageToMap(p *model.Page) map[string]any {
	return entity.PackPage(p, h.idGen)
}

func notFound(c echo.Context) error {
	return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_PAGE", "No such page.", "222120c0-3ead-4528-811b-b96f233388d7"))
}

func nameConflict(c echo.Context) error {
	return c.JSON(http.StatusBadRequest, apierr.Error("NAME_ALREADY_EXISTS", "The page name is already in use.", "4650348e-301c-499a-83c9-6aa988c66bc1"))
}

func alreadyLiked(c echo.Context) error {
	return c.JSON(http.StatusBadRequest, apierr.Error("ALREADY_LIKED", "You already liked that page.", "cc98a8a2-0dc3-4123-b198-62c71df18ed3"))
}

func notLiked(c echo.Context) error {
	return c.JSON(http.StatusBadRequest, apierr.Error("NOT_LIKED", "You have not liked that page.", "f5e586b0-ce93-4050-b0e3-7f31af5259ee"))
}
