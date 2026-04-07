package notes

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/core/reaction"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// ReactionCreateRequest is the request body for notes/reactions/create.
type ReactionCreateRequest struct {
	NoteID   string `json:"noteId"`
	Reaction string `json:"reaction"`
}

// ReactionsCreate handles POST /api/notes/reactions/create.
func (h *Handler) ReactionsCreate(c echo.Context) error {
	user := middleware.GetUser(c)

	var req ReactionCreateRequest
	if err := c.Bind(&req); err != nil || req.NoteID == "" {
		return invalidParam(c)
	}

	_, err := h.reactionService.Create(user, req.NoteID, req.Reaction)
	if err != nil {
		switch {
		case errors.Is(err, reaction.ErrNoteNotFound):
			return noSuchNote(c)
		case errors.Is(err, reaction.ErrNoteNotVisible):
			return c.JSON(http.StatusForbidden, map[string]any{
				"error": map[string]any{
					"message": "You can not see this note.",
					"code":    "ACCESS_DENIED",
					"id":      "fe8d7103-0ea8-4ec3-814d-f8b401dc69e9",
				},
			})
		case errors.Is(err, reaction.ErrAlreadyReacted):
			return c.JSON(http.StatusBadRequest, map[string]any{
				"error": map[string]any{
					"message": "You are already reacting to that note.",
					"code":    "ALREADY_REACTED",
					"id":      "71efcf98-86d6-4e2b-b2ad-9d032369366b",
				},
			})
		case errors.Is(err, reaction.ErrCannotReactToPureRenote):
			return c.JSON(http.StatusBadRequest, map[string]any{
				"error": map[string]any{
					"message": "You can not react to a pure Renote.",
					"code":    "CANNOT_REACT_TO_RENOTE",
					"id":      "eaccdc08-ddef-43fe-908f-d108faad57f5",
				},
			})
		case errors.Is(err, reaction.ErrBlocked):
			return c.JSON(http.StatusForbidden, map[string]any{
				"error": map[string]any{
					"message": "You are blocked by that user.",
					"code":    "BLOCKED",
					"id":      "e70412a4-7197-4726-8e74-f3e0deb92aa7",
				},
			})
		}
		return internalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}

// ReactionDeleteRequest is the request body for notes/reactions/delete.
type ReactionDeleteRequest struct {
	NoteID string `json:"noteId"`
}

// ReactionsDelete handles POST /api/notes/reactions/delete.
func (h *Handler) ReactionsDelete(c echo.Context) error {
	user := middleware.GetUser(c)

	var req ReactionDeleteRequest
	if err := c.Bind(&req); err != nil || req.NoteID == "" {
		return invalidParam(c)
	}

	if err := h.reactionService.Delete(user, req.NoteID); err != nil {
		switch {
		case errors.Is(err, reaction.ErrNoteNotFound):
			return noSuchNote(c)
		case errors.Is(err, reaction.ErrReactionNotFound):
			return c.JSON(http.StatusNotFound, map[string]any{
				"error": map[string]any{
					"message": "You are not reacting to that note.",
					"code":    "NOT_REACTED",
					"id":      "92f4426d-4196-4125-aa5b-02943e2ec8fc",
				},
			})
		}
		return internalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}

// ReactionsListRequest is the request body for notes/reactions.
type ReactionsListRequest struct {
	NoteID  string `json:"noteId"`
	Type    string `json:"type"`
	Limit   int    `json:"limit"`
	SinceID string `json:"sinceId"`
	UntilID string `json:"untilId"`
}

// Reactions handles POST /api/notes/reactions.
func (h *Handler) Reactions(c echo.Context) error {
	var req ReactionsListRequest
	if err := c.Bind(&req); err != nil || req.NoteID == "" {
		return invalidParam(c)
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Limit > 100 {
		req.Limit = 100
	}

	viewer := middleware.GetUser(c)
	rows, err := h.reactionService.List(viewer, req.NoteID, req.UntilID, req.SinceID, req.Limit, req.Type)
	if err != nil {
		switch {
		case errors.Is(err, reaction.ErrNoteNotFound), errors.Is(err, reaction.ErrNoteNotVisible):
			return noSuchNote(c)
		}
		return internalError(c)
	}

	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		createdAt := ""
		if t, err := h.idGen.ParseTime(r.ID); err == nil {
			createdAt = t.UTC().Format("2006-01-02T15:04:05.000Z")
		}
		out = append(out, map[string]any{
			"id":        r.ID,
			"createdAt": createdAt,
			"user":      map[string]any{"id": r.UserID},
			"type":      r.Reaction,
		})
	}
	return c.JSON(http.StatusOK, out)
}
