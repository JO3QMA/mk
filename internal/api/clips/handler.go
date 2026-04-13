// Package clips provides /api/clips/* endpoints.
package clips

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	coreclip "github.com/shiroha-a/mk/internal/core/clip"
	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// Handler handles clip-related API endpoints.
type Handler struct {
	svc          *coreclip.Service
	idGen        id.Generator
	favoriteRepo ClipFavoriteRepository
}

// NewHandler creates a new clips Handler.
func NewHandler(svc *coreclip.Service, idGen id.Generator) *Handler {
	return &Handler{svc: svc, idGen: idGen}
}

// CreateRequest is the request body for clips/create.
type CreateRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	IsPublic    bool    `json:"isPublic"`
}

// Create handles POST /api/clips/create.
func (h *Handler) Create(c echo.Context) error {
	user := middleware.GetUser(c)
	var req CreateRequest
	if err := c.Bind(&req); err != nil || req.Name == "" {
		return invalidParam(c)
	}
	cl, err := h.svc.Create(coreclip.CreateInput{
		OwnerID:     user.ID,
		Name:        req.Name,
		Description: req.Description,
		IsPublic:    req.IsPublic,
	})
	if err != nil {
		return internalError(c)
	}
	return c.JSON(http.StatusOK, clipToMap(cl))
}

// ShowRequest is the request body for clips/show.
type ShowRequest struct {
	ClipID string `json:"clipId"`
}

// Show handles POST /api/clips/show.
func (h *Handler) Show(c echo.Context) error {
	user := middleware.GetUser(c)
	var req ShowRequest
	if err := c.Bind(&req); err != nil || req.ClipID == "" {
		return invalidParam(c)
	}
	requesterID := ""
	if user != nil {
		requesterID = user.ID
	}
	cl, err := h.svc.Show(requesterID, req.ClipID)
	if err != nil {
		if errors.Is(err, coreclip.ErrAccessDenied) {
			return accessDenied(c)
		}
		return notFound(c)
	}
	return c.JSON(http.StatusOK, clipToMap(cl))
}

// UpdateRequest is the request body for clips/update.
type UpdateRequest struct {
	ClipID      string  `json:"clipId"`
	Name        *string `json:"name"`
	Description *string `json:"description"`
	IsPublic    *bool   `json:"isPublic"`
}

// Update handles POST /api/clips/update.
func (h *Handler) Update(c echo.Context) error {
	user := middleware.GetUser(c)
	var req UpdateRequest
	if err := c.Bind(&req); err != nil || req.ClipID == "" {
		return invalidParam(c)
	}
	in := coreclip.UpdateInput{
		Name:     req.Name,
		IsPublic: req.IsPublic,
	}
	if req.Description != nil {
		desc := req.Description
		in.Description = &desc
	}
	cl, err := h.svc.Update(user.ID, req.ClipID, in)
	if err != nil {
		switch {
		case errors.Is(err, coreclip.ErrClipNotFound):
			return notFound(c)
		case errors.Is(err, coreclip.ErrAccessDenied):
			return accessDenied(c)
		case errors.Is(err, coreclip.ErrClipNameRequired):
			return invalidParam(c)
		}
		return internalError(c)
	}
	return c.JSON(http.StatusOK, clipToMap(cl))
}

// DeleteRequest is the request body for clips/delete.
type DeleteRequest struct {
	ClipID string `json:"clipId"`
}

// Delete handles POST /api/clips/delete.
func (h *Handler) Delete(c echo.Context) error {
	user := middleware.GetUser(c)
	var req DeleteRequest
	if err := c.Bind(&req); err != nil || req.ClipID == "" {
		return invalidParam(c)
	}
	if err := h.svc.Delete(user.ID, req.ClipID); err != nil {
		switch {
		case errors.Is(err, coreclip.ErrClipNotFound):
			return notFound(c)
		case errors.Is(err, coreclip.ErrAccessDenied):
			return accessDenied(c)
		}
		return internalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}

// ListRequest is the request body for clips/list.
type ListRequest struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// List handles POST /api/clips/list.
func (h *Handler) List(c echo.Context) error {
	user := middleware.GetUser(c)
	var req ListRequest
	if err := c.Bind(&req); err != nil {
		return invalidParam(c)
	}
	rows, err := h.svc.ListByUser(user.ID, req.Limit, req.Offset)
	if err != nil {
		return internalError(c)
	}
	out := make([]map[string]any, 0, len(rows))
	for _, cl := range rows {
		out = append(out, clipToMap(cl))
	}
	return c.JSON(http.StatusOK, out)
}

// AddNoteRequest is the request body for clips/add-note.
type AddNoteRequest struct {
	ClipID string `json:"clipId"`
	NoteID string `json:"noteId"`
}

// AddNote handles POST /api/clips/add-note.
func (h *Handler) AddNote(c echo.Context) error {
	user := middleware.GetUser(c)
	var req AddNoteRequest
	if err := c.Bind(&req); err != nil || req.ClipID == "" || req.NoteID == "" {
		return invalidParam(c)
	}
	if err := h.svc.AddNote(user.ID, req.ClipID, req.NoteID); err != nil {
		switch {
		case errors.Is(err, coreclip.ErrClipNotFound):
			return notFound(c)
		case errors.Is(err, coreclip.ErrAccessDenied):
			return accessDenied(c)
		case errors.Is(err, coreclip.ErrNoteNotFound):
			return notFoundNote(c)
		case errors.Is(err, coreclip.ErrAlreadyClipped):
			return alreadyClipped(c)
		}
		return internalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}

// RemoveNoteRequest is the request body for clips/remove-note.
type RemoveNoteRequest struct {
	ClipID string `json:"clipId"`
	NoteID string `json:"noteId"`
}

// RemoveNote handles POST /api/clips/remove-note.
func (h *Handler) RemoveNote(c echo.Context) error {
	user := middleware.GetUser(c)
	var req RemoveNoteRequest
	if err := c.Bind(&req); err != nil || req.ClipID == "" || req.NoteID == "" {
		return invalidParam(c)
	}
	if err := h.svc.RemoveNote(user.ID, req.ClipID, req.NoteID); err != nil {
		switch {
		case errors.Is(err, coreclip.ErrClipNotFound):
			return notFound(c)
		case errors.Is(err, coreclip.ErrAccessDenied):
			return accessDenied(c)
		case errors.Is(err, coreclip.ErrNotClipped):
			return notClipped(c)
		}
		return internalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}

// NotesRequest is the request body for clips/notes.
type NotesRequest struct {
	ClipID  string `json:"clipId"`
	UntilID string `json:"untilId"`
	SinceID string `json:"sinceId"`
	Limit   int    `json:"limit"`
}

// Notes handles POST /api/clips/notes.
func (h *Handler) Notes(c echo.Context) error {
	user := middleware.GetUser(c)
	var req NotesRequest
	if err := c.Bind(&req); err != nil || req.ClipID == "" {
		return invalidParam(c)
	}
	requesterID := ""
	if user != nil {
		requesterID = user.ID
	}
	notes, err := h.svc.Notes(requesterID, req.ClipID, req.UntilID, req.SinceID, req.Limit)
	if err != nil {
		switch {
		case errors.Is(err, coreclip.ErrClipNotFound):
			return notFound(c)
		case errors.Is(err, coreclip.ErrAccessDenied):
			return accessDenied(c)
		}
		return internalError(c)
	}
	out := make([]any, 0, len(notes))
	for _, n := range notes {
		out = append(out, entity.PackNote(n, h.idGen))
	}
	return c.JSON(http.StatusOK, out)
}

func clipToMap(cl *model.Clip) map[string]any {
	return map[string]any{
		"id":            cl.ID,
		"userId":        cl.UserID,
		"name":          cl.Name,
		"description":   cl.Description,
		"isPublic":      cl.IsPublic,
		"notesCount":    cl.NotesCount,
		"lastClippedAt": cl.LastClippedAt,
	}
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

func notFound(c echo.Context) error {
	return c.JSON(http.StatusNotFound, map[string]any{
		"error": map[string]any{
			"message": "No such clip.",
			"code":    "NO_SUCH_CLIP",
			"id":      "f4a9c0c6-a6c3-4a07-8e6b-0a4f2b1a27e9",
		},
	})
}

func accessDenied(c echo.Context) error {
	return c.JSON(http.StatusForbidden, map[string]any{
		"error": map[string]any{
			"message": "Access denied.",
			"code":    "ACCESS_DENIED",
			"id":      "1fb7cb09-d46a-4fff-b8df-057708cce513",
		},
	})
}

func notFoundNote(c echo.Context) error {
	return c.JSON(http.StatusNotFound, map[string]any{
		"error": map[string]any{
			"message": "No such note.",
			"code":    "NO_SUCH_NOTE",
			"id":      "24fcbfc6-2e37-42b6-8388-c29b3272571530",
		},
	})
}

func alreadyClipped(c echo.Context) error {
	return c.JSON(http.StatusBadRequest, map[string]any{
		"error": map[string]any{
			"message": "The note has already been clipped.",
			"code":    "ALREADY_CLIPPED",
			"id":      "734806c4-542c-463a-9311-15c512803965",
		},
	})
}

func notClipped(c echo.Context) error {
	return c.JSON(http.StatusBadRequest, map[string]any{
		"error": map[string]any{
			"message": "The note is not clipped.",
			"code":    "NOT_CLIPPED",
			"id":      "aff017de-190e-434b-893e-33a9ff5049d8",
		},
	})
}
