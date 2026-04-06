package notes

import (
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// Handler handles note-related API endpoints.
type Handler struct {
	noteRepo repository.NoteRepository
	pollRepo repository.PollRepository
	idGen    id.Generator
}

// NewHandler creates a new notes Handler.
func NewHandler(noteRepo repository.NoteRepository, pollRepo repository.PollRepository, idGen id.Generator) *Handler {
	return &Handler{noteRepo: noteRepo, pollRepo: pollRepo, idGen: idGen}
}

// CreateRequest is the request body for notes/create.
type CreateRequest struct {
	Visibility         string       `json:"visibility"`
	VisibleUserIDs     []string     `json:"visibleUserIds"`
	CW                 *string      `json:"cw"`
	Text               *string      `json:"text"`
	LocalOnly          bool         `json:"localOnly"`
	ReactionAcceptance *string      `json:"reactionAcceptance"`
	FileIDs            []string     `json:"fileIds"`
	ReplyID            *string      `json:"replyId"`
	RenoteID           *string      `json:"renoteId"`
	ChannelID          *string      `json:"channelId"`
	Poll               *PollRequest `json:"poll"`
}

// PollRequest is the poll part of a create note request.
type PollRequest struct {
	Choices      []string `json:"choices"`
	Multiple     bool     `json:"multiple"`
	ExpiresAt    *int64   `json:"expiresAt"`
	ExpiredAfter *int64   `json:"expiredAfter"`
}

// Create handles POST /api/notes/create.
func (h *Handler) Create(c echo.Context) error {
	user := middleware.GetUser(c)

	var req CreateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error": map[string]any{
				"message": "Invalid param.",
				"code":    "INVALID_PARAM",
				"id":      "3d81ceae-475f-4600-b2a8-2bc116157532",
			},
		})
	}

	if req.Text == nil && req.RenoteID == nil && len(req.FileIDs) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error": map[string]any{
				"message": "Text, fileIds, or renoteId is required.",
				"code":    "INVALID_PARAM",
				"id":      "3d81ceae-475f-4600-b2a8-2bc116157532",
			},
		})
	}

	visibility := model.NoteVisibilityPublic
	if req.Visibility != "" {
		visibility = model.NoteVisibility(req.Visibility)
	}

	now := time.Now()
	noteID := h.idGen.Generate(now)

	note := model.Note{
		ID:                 noteID,
		UserID:             user.ID,
		Text:               req.Text,
		CW:                 req.CW,
		Visibility:         visibility,
		LocalOnly:          req.LocalOnly,
		ReactionAcceptance: req.ReactionAcceptance,
		ReplyID:            req.ReplyID,
		RenoteID:           req.RenoteID,
		ChannelID:          req.ChannelID,
		FileIDs:            req.FileIDs,
		UserHost:           user.Host,
	}

	if req.VisibleUserIDs != nil {
		note.VisibleUserIDs = req.VisibleUserIDs
	}

	// メンション解析
	if req.Text != nil {
		note.Mentions = extractMentions(*req.Text)
	}

	if err := h.noteRepo.Create(&note); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{
			"error": map[string]any{
				"message": "Internal error.",
				"code":    "INTERNAL_ERROR",
				"id":      "5d37dbcb-891e-41ca-a3d6-e690c97775ac",
			},
		})
	}

	// Poll作成
	if req.Poll != nil && len(req.Poll.Choices) > 0 {
		votes := make([]int64, len(req.Poll.Choices))
		poll := model.Poll{
			NoteID:         noteID,
			Multiple:       req.Poll.Multiple,
			Choices:        req.Poll.Choices,
			Votes:          votes,
			NoteVisibility: visibility,
			UserID:         user.ID,
			UserHost:       user.Host,
			ChannelID:      req.ChannelID,
		}
		if req.Poll.ExpiresAt != nil {
			t := time.UnixMilli(*req.Poll.ExpiresAt)
			poll.ExpiresAt = &t
		}
		_ = h.pollRepo.Create(&poll)
		_ = h.noteRepo.Update(&note, "hasPoll", true)
		note.HasPoll = true
	}

	// Userをpreloadして返す
	loaded, _ := h.noteRepo.FindByIDWithUser(noteID)
	if loaded != nil {
		return c.JSON(http.StatusOK, map[string]any{
			"createdNote": entity.PackNote(loaded, h.idGen),
		})
	}

	note.User = user
	return c.JSON(http.StatusOK, map[string]any{
		"createdNote": entity.PackNote(&note, h.idGen),
	})
}

// ShowRequest is the request body for notes/show.
type ShowRequest struct {
	NoteID string `json:"noteId"`
}

// Show handles POST /api/notes/show.
func (h *Handler) Show(c echo.Context) error {
	var req ShowRequest
	if err := c.Bind(&req); err != nil || req.NoteID == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error": map[string]any{
				"message": "Invalid param.",
				"code":    "INVALID_PARAM",
				"id":      "3d81ceae-475f-4600-b2a8-2bc116157532",
			},
		})
	}

	note, err := h.noteRepo.FindByIDWithUser(req.NoteID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]any{
			"error": map[string]any{
				"message": "No such note.",
				"code":    "NO_SUCH_NOTE",
				"id":      "24fcbfc6-2e37-42b6-8388-c29b3272571530",
			},
		})
	}

	return c.JSON(http.StatusOK, entity.PackNote(note, h.idGen))
}

// DeleteRequest is the request body for notes/delete.
type DeleteRequest struct {
	NoteID string `json:"noteId"`
}

// Delete handles POST /api/notes/delete.
func (h *Handler) Delete(c echo.Context) error {
	user := middleware.GetUser(c)

	var req DeleteRequest
	if err := c.Bind(&req); err != nil || req.NoteID == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error": map[string]any{
				"message": "Invalid param.",
				"code":    "INVALID_PARAM",
				"id":      "3d81ceae-475f-4600-b2a8-2bc116157532",
			},
		})
	}

	note, err := h.noteRepo.FindByID(req.NoteID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]any{
			"error": map[string]any{
				"message": "No such note.",
				"code":    "NO_SUCH_NOTE",
				"id":      "490be23f-8c1f-4796-819f-94cb4f9d1630",
			},
		})
	}

	if note.UserID != user.ID {
		return c.JSON(http.StatusForbidden, map[string]any{
			"error": map[string]any{
				"message": "You are not the author of this note.",
				"code":    "ACCESS_DENIED",
				"id":      "fe8d7103-0ea8-4ec3-814d-f8b401dc69e9",
			},
		})
	}

	_ = h.noteRepo.Delete(note)

	return c.NoContent(http.StatusNoContent)
}

// extractMentions は簡易的なメンション抽出（後で完全実装する）
func extractMentions(text string) []string {
	var mentions []string
	words := strings.Fields(text)
	for _, w := range words {
		if strings.HasPrefix(w, "@") && len(w) > 1 {
			mentions = append(mentions, strings.TrimPrefix(w, "@"))
		}
	}
	return mentions
}
