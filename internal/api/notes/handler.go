package notes

import (
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/core/note"
	"github.com/shiroha-a/mk/internal/core/poll"
	"github.com/shiroha-a/mk/internal/core/reaction"
	"github.com/shiroha-a/mk/internal/core/timeline"
	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// Handler handles note-related API endpoints.
type Handler struct {
	noteRepo        repository.NoteRepository
	createService   *note.CreateService
	deleteService   *note.DeleteService
	queryService    *note.QueryService
	timelineService *timeline.Service
	reactionService *reaction.Service
	pollService     *poll.Service
	idGen           id.Generator
}

// NewHandler creates a new notes Handler.
// queryService/timelineService/reactionService/pollServiceがnilの場合は
// 対応するエンドポイントは利用不可となる (テストで一部だけ初期化する用途を許容する)。
func NewHandler(
	noteRepo repository.NoteRepository,
	createService *note.CreateService,
	deleteService *note.DeleteService,
	queryService *note.QueryService,
	timelineService *timeline.Service,
	reactionService *reaction.Service,
	pollService *poll.Service,
	idGen id.Generator,
) *Handler {
	return &Handler{
		noteRepo:        noteRepo,
		createService:   createService,
		deleteService:   deleteService,
		queryService:    queryService,
		timelineService: timelineService,
		reactionService: reactionService,
		pollService:     pollService,
		idGen:           idGen,
	}
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
		return invalidParam(c)
	}

	in := note.CreateInput{
		User:               user,
		Text:               req.Text,
		CW:                 req.CW,
		Visibility:         model.NoteVisibility(req.Visibility),
		VisibleUserIDs:     req.VisibleUserIDs,
		LocalOnly:          req.LocalOnly,
		ReactionAcceptance: req.ReactionAcceptance,
		FileIDs:            req.FileIDs,
		ReplyID:            req.ReplyID,
		RenoteID:           req.RenoteID,
		ChannelID:          req.ChannelID,
	}

	if req.Poll != nil {
		in.Poll = &note.PollInput{
			Choices:  req.Poll.Choices,
			Multiple: req.Poll.Multiple,
		}
		if req.Poll.ExpiresAt != nil {
			t := time.UnixMilli(*req.Poll.ExpiresAt)
			in.Poll.ExpiresAt = &t
		}
	}

	created, err := h.createService.Create(in)
	if err != nil {
		switch {
		case errors.Is(err, note.ErrNoteContentRequired):
			return c.JSON(http.StatusBadRequest, map[string]any{
				"error": map[string]any{
					"message": "Text, fileIds, or renoteId is required.",
					"code":    "INVALID_PARAM",
					"id":      "3d81ceae-475f-4600-b2a8-2bc116157532",
				},
			})
		case errors.Is(err, note.ErrReplyTargetNotFound), errors.Is(err, note.ErrRenoteTargetNotFound):
			return c.JSON(http.StatusNotFound, map[string]any{
				"error": map[string]any{
					"message": "No such note.",
					"code":    "NO_SUCH_NOTE",
					"id":      "eef6c173-3010-4a23-8674-7c4fcaeba719",
				},
			})
		case errors.Is(err, note.ErrCannotReplyToInvisibleNote), errors.Is(err, note.ErrCannotRenoteInvisibleNote):
			return c.JSON(http.StatusForbidden, map[string]any{
				"error": map[string]any{
					"message": "You can not see this note.",
					"code":    "CANNOT_REPLY_TO_INVISIBLE_NOTE",
					"id":      "44b07c37-2deb-4b34-9ec9-b1deeed42f0d",
				},
			})
		}
		return internalError(c)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"createdNote": entity.PackNote(created, h.idGen),
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
		return invalidParam(c)
	}

	viewer := middleware.GetUser(c)
	n, err := h.lookupVisible(viewer, req.NoteID)
	if err != nil {
		return noSuchNote(c)
	}

	return c.JSON(http.StatusOK, entity.PackNote(n, h.idGen))
}

// lookupVisible fetches the note via QueryService when available, otherwise
// falls back to a direct repository lookup. テストでQueryServiceなしで初期化
// された場合の後方互換のためにフォールバックを残している。
func (h *Handler) lookupVisible(viewer *model.User, noteID string) (*model.Note, error) {
	if h.queryService != nil {
		return h.queryService.Show(viewer, noteID)
	}
	return h.noteRepo.FindByIDWithUser(noteID)
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
		return invalidParam(c)
	}

	if err := h.deleteService.Delete(user, req.NoteID); err != nil {
		switch {
		case errors.Is(err, note.ErrNoteNotFound):
			return c.JSON(http.StatusNotFound, map[string]any{
				"error": map[string]any{
					"message": "No such note.",
					"code":    "NO_SUCH_NOTE",
					"id":      "490be23f-8c1f-4796-819f-94cb4f9d1630",
				},
			})
		case errors.Is(err, note.ErrNoteAccessDenied):
			return c.JSON(http.StatusForbidden, map[string]any{
				"error": map[string]any{
					"message": "You are not the author of this note.",
					"code":    "ACCESS_DENIED",
					"id":      "fe8d7103-0ea8-4ec3-814d-f8b401dc69e9",
				},
			})
		default:
			return internalError(c)
		}
	}

	return c.NoContent(http.StatusNoContent)
}

// listRequest is the common pagination request shared by renotes/replies/children.
type listRequest struct {
	NoteID  string `json:"noteId"`
	Limit   int    `json:"limit"`
	SinceID string `json:"sinceId"`
	UntilID string `json:"untilId"`
}

func (r *listRequest) normalize() {
	if r.Limit <= 0 {
		r.Limit = 10
	}
	if r.Limit > 100 {
		r.Limit = 100
	}
}

// Renotes handles POST /api/notes/renotes.
func (h *Handler) Renotes(c echo.Context) error {
	return h.serveList(c, func(viewer *model.User, req listRequest) ([]*model.Note, error) {
		return h.queryService.ListRenotes(viewer, req.NoteID, req.UntilID, req.SinceID, req.Limit)
	})
}

// Replies handles POST /api/notes/replies.
func (h *Handler) Replies(c echo.Context) error {
	return h.serveList(c, func(viewer *model.User, req listRequest) ([]*model.Note, error) {
		return h.queryService.ListReplies(viewer, req.NoteID, req.UntilID, req.SinceID, req.Limit)
	})
}

// Children handles POST /api/notes/children.
func (h *Handler) Children(c echo.Context) error {
	return h.serveList(c, func(viewer *model.User, req listRequest) ([]*model.Note, error) {
		return h.queryService.ListChildren(viewer, req.NoteID, req.UntilID, req.SinceID, req.Limit)
	})
}

func (h *Handler) serveList(c echo.Context, fn func(*model.User, listRequest) ([]*model.Note, error)) error {
	var req listRequest
	if err := c.Bind(&req); err != nil || req.NoteID == "" {
		return invalidParam(c)
	}
	req.normalize()

	viewer := middleware.GetUser(c)
	notes, err := fn(viewer, req)
	if err != nil {
		if errors.Is(err, note.ErrNoteNotFound) {
			return noSuchNote(c)
		}
		return internalError(c)
	}
	return c.JSON(http.StatusOK, h.packMany(notes))
}

// SearchRequest is the request body for notes/search.
type SearchRequest struct {
	Query   string `json:"query"`
	Limit   int    `json:"limit"`
	SinceID string `json:"sinceId"`
	UntilID string `json:"untilId"`
}

// Search handles POST /api/notes/search.
func (h *Handler) Search(c echo.Context) error {
	var req SearchRequest
	if err := c.Bind(&req); err != nil {
		return invalidParam(c)
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Limit > 100 {
		req.Limit = 100
	}

	viewer := middleware.GetUser(c)
	notes, err := h.queryService.Search(viewer, req.Query, req.UntilID, req.SinceID, req.Limit)
	if err != nil {
		// 空クエリは invalidParam として返す。
		// それ以外のエラー (DB障害など) はinternalErrorで返す。
		if errors.Is(err, note.ErrEmptySearchQuery) {
			return invalidParam(c)
		}
		return internalError(c)
	}
	return c.JSON(http.StatusOK, h.packMany(notes))
}

// State handles POST /api/notes/state.
func (h *Handler) State(c echo.Context) error {
	var req ShowRequest
	if err := c.Bind(&req); err != nil || req.NoteID == "" {
		return invalidParam(c)
	}
	viewer := middleware.GetUser(c)
	st, err := h.queryService.State(viewer, req.NoteID)
	if err != nil {
		// 現状QueryService.StateはErrNoteNotFound以外を返さない
		return noSuchNote(c)
	}
	return c.JSON(http.StatusOK, st)
}

// ConversationRequest is the request body for notes/conversation.
type ConversationRequest struct {
	NoteID string `json:"noteId"`
	Limit  int    `json:"limit"`
}

// Conversation handles POST /api/notes/conversation.
func (h *Handler) Conversation(c echo.Context) error {
	var req ConversationRequest
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
	notes, err := h.queryService.Conversation(viewer, req.NoteID, req.Limit)
	if err != nil {
		// 現状QueryService.ConversationはErrNoteNotFound以外を返さない
		return noSuchNote(c)
	}
	return c.JSON(http.StatusOK, h.packMany(notes))
}

// packMany serializes a list of notes into NoteEntity objects.
func (h *Handler) packMany(notes []*model.Note) []entity.NoteEntity {
	out := make([]entity.NoteEntity, 0, len(notes))
	for _, n := range notes {
		out = append(out, entity.PackNote(n, h.idGen))
	}
	return out
}

func noSuchNote(c echo.Context) error {
	return c.JSON(http.StatusNotFound, map[string]any{
		"error": map[string]any{
			"message": "No such note.",
			"code":    "NO_SUCH_NOTE",
			"id":      "24fcbfc6-2e37-42b6-8388-c29b3272571530",
		},
	})
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
