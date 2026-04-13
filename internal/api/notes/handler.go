package notes

import (
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/core/note"
	"github.com/shiroha-a/mk/internal/core/poll"
	"github.com/shiroha-a/mk/internal/core/reaction"
	"github.com/shiroha-a/mk/internal/core/search"
	"github.com/shiroha-a/mk/internal/core/timeline"
	"github.com/shiroha-a/mk/internal/core/translate"
	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// Handler handles note-related API endpoints.
type Handler struct {
	noteRepo         repository.NoteRepository
	createService    *note.CreateService
	deleteService    *note.DeleteService
	queryService     *note.QueryService
	timelineService  *timeline.Service
	reactionService  *reaction.Service
	pollService      *poll.Service
	searchService    *search.Service
	idGen            id.Generator
	favoriteRepo     repository.NoteFavoriteRepository
	driveFileRepo    repository.DriveFileRepository
	draftRepo        repository.NoteDraftRepository
	noteReactionRepo repository.NoteReactionRepository
	channelRepo      repository.ChannelRepository
	// ugcVisibility controls what unauthenticated visitors can see.
	// "all" (default), "local", "none"
	ugcVisibility string
	translator    *translate.DeepLClient
}

// SetTranslator attaches a DeepL translator for /api/notes/translate.
func (h *Handler) SetTranslator(t *translate.DeepLClient) {
	h.translator = t
}

// SetUGCVisibility sets the visitor content visibility policy from meta.
func (h *Handler) SetUGCVisibility(v string) {
	h.ugcVisibility = v
}

// SetDriveFileRepo attaches a DriveFileRepository for file resolution.
func (h *Handler) SetDriveFileRepo(r repository.DriveFileRepository) {
	h.driveFileRepo = r
}

// SetNoteReactionRepo attaches a NoteReactionRepository for myReaction resolution.
func (h *Handler) SetNoteReactionRepo(r repository.NoteReactionRepository) {
	h.noteReactionRepo = r
}

// SetChannelRepo attaches a ChannelRepository for channel resolution.
func (h *Handler) SetChannelRepo(r repository.ChannelRepository) {
	h.channelRepo = r
}

// NewHandler creates a new notes Handler.
// queryService/timelineService/reactionService/pollService/searchService が
// nil の場合、それぞれの依存に対応するエンドポイントは利用不可となる
// (テストで一部だけ初期化する用途を許容する)。
func NewHandler(
	noteRepo repository.NoteRepository,
	createService *note.CreateService,
	deleteService *note.DeleteService,
	queryService *note.QueryService,
	timelineService *timeline.Service,
	reactionService *reaction.Service,
	pollService *poll.Service,
	searchService *search.Service,
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
		searchService:   searchService,
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

	packed := entity.PackNote(created, h.idGen)
	s := []entity.NoteEntity{packed}
	h.resolveFiles(s)
	h.resolveViewerFields(s, user)
	return c.JSON(http.StatusOK, map[string]any{
		"createdNote": s[0],
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

	packed := entity.PackNote(n, h.idGen)
	s := []entity.NoteEntity{packed}
	h.resolveFiles(s)
	h.resolveViewerFields(s, viewer)
	return c.JSON(http.StatusOK, s[0])
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
	return c.JSON(http.StatusOK, h.packMany(notes, viewer))
}

// SearchRequest is the request body for notes/search.
//
// Misskey 本家と互換のフィールド構成。`sinceDate` / `untilDate` (unix milli)
// が指定されたときは ID generator で対応する note ID に変換し、`sinceId` /
// `untilId` のフォールバックとして使う。`host == "."` はローカル限定検索。
type SearchRequest struct {
	Query     string `json:"query"`
	Limit     int    `json:"limit"`
	SinceID   string `json:"sinceId"`
	UntilID   string `json:"untilId"`
	SinceDate *int64 `json:"sinceDate"`
	UntilDate *int64 `json:"untilDate"`
	UserID    string `json:"userId"`
	ChannelID string `json:"channelId"`
	Host      string `json:"host"`
}

// Search handles POST /api/notes/search.
func (h *Handler) Search(c echo.Context) error {
	var req SearchRequest
	if err := c.Bind(&req); err != nil {
		return invalidParam(c)
	}
	if h.searchService == nil {
		return internalError(c)
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Limit > 100 {
		req.Limit = 100
	}

	untilID := req.UntilID
	if untilID == "" && req.UntilDate != nil && h.idGen != nil {
		untilID = h.idGen.Generate(time.UnixMilli(*req.UntilDate))
	}
	sinceID := req.SinceID
	if sinceID == "" && req.SinceDate != nil && h.idGen != nil {
		sinceID = h.idGen.Generate(time.UnixMilli(*req.SinceDate))
	}

	viewer := middleware.GetUser(c)
	notes, err := h.searchService.SearchNote(
		viewer,
		req.Query,
		search.SearchOpts{
			UserID:    req.UserID,
			ChannelID: req.ChannelID,
			Host:      req.Host,
		},
		search.Pagination{
			UntilID: untilID,
			SinceID: sinceID,
			Limit:   req.Limit,
		},
	)
	if err != nil {
		// 空クエリは invalidParam として返す。
		// それ以外のエラー (DB障害など) はinternalErrorで返す。
		if errors.Is(err, search.ErrEmptyQuery) {
			return invalidParam(c)
		}
		return internalError(c)
	}
	return c.JSON(http.StatusOK, h.packMany(notes, viewer))
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
	return c.JSON(http.StatusOK, h.packMany(notes, viewer))
}

// packMany serializes a list of notes into NoteEntity objects.
// driveFileRepoが設定されている場合、ファイル情報を解決してFilesに含める。
// viewerがnon-nilの場合、myReactionなどのviewer依存フィールドも解決する。
func (h *Handler) packMany(notes []*model.Note, viewer *model.User) []entity.NoteEntity {
	out := make([]entity.NoteEntity, 0, len(notes))
	for _, n := range notes {
		out = append(out, entity.PackNote(n, h.idGen))
	}
	h.resolveFiles(out)
	h.resolveViewerFields(out, viewer)
	return out
}

// resolveFiles collects all fileIds from notes, fetches DriveFiles in bulk,
// and populates the Files field of each NoteEntity.
func (h *Handler) resolveFiles(notes []entity.NoteEntity) {
	if h.driveFileRepo == nil {
		return
	}
	// 全fileIDsを収集
	var allIDs []string
	for _, n := range notes {
		allIDs = append(allIDs, n.FileIDs...)
	}
	if len(allIDs) == 0 {
		return
	}
	files, err := h.driveFileRepo.FindByIDs(allIDs)
	if err != nil || len(files) == 0 {
		return
	}
	// IDでマップ化
	fileMap := make(map[string]*model.DriveFile, len(files))
	for _, f := range files {
		fileMap[f.ID] = f
	}
	// 各ノートのFilesに設定
	for i := range notes {
		packed := make([]any, 0, len(notes[i].FileIDs))
		for _, fid := range notes[i].FileIDs {
			if f, ok := fileMap[fid]; ok {
				packed = append(packed, entity.PackDriveFile(f, h.idGen))
			}
		}
		notes[i].Files = packed
	}
}

// resolveViewerFields populates viewer-dependent fields (MyReaction, Channel)
// on packed NoteEntities. viewerがnilの場合はChannel解決のみ行う。
func (h *Handler) resolveViewerFields(notes []entity.NoteEntity, viewer *model.User) {
	if len(notes) == 0 {
		return
	}

	// MyReaction: viewerが認証済みの場合にバッチ取得
	if viewer != nil && h.noteReactionRepo != nil {
		noteIDs := make([]string, len(notes))
		for i := range notes {
			noteIDs[i] = notes[i].ID
		}
		reactionMap, err := h.noteReactionRepo.FindByUserAndNoteIDs(viewer.ID, noteIDs)
		if err == nil {
			for i := range notes {
				if r, ok := reactionMap[notes[i].ID]; ok {
					notes[i].MyReaction = &r.Reaction
				}
			}
		}
	}

	// Channel: ChannelIDがnon-nilのノートについてバッチ取得
	if h.channelRepo != nil {
		var channelIDs []string
		for _, n := range notes {
			if n.ChannelID != nil && *n.ChannelID != "" {
				channelIDs = append(channelIDs, *n.ChannelID)
			}
		}
		if len(channelIDs) > 0 {
			channels, err := h.channelRepo.FindByIDs(channelIDs)
			if err == nil {
				chMap := make(map[string]*model.Channel, len(channels))
				for _, ch := range channels {
					chMap[ch.ID] = ch
				}
				for i := range notes {
					if notes[i].ChannelID != nil {
						if ch, ok := chMap[*notes[i].ChannelID]; ok {
							notes[i].Channel = &entity.ChannelLite{
								ID:                    ch.ID,
								Name:                  ch.Name,
								Color:                 ch.Color,
								IsSensitive:           ch.IsSensitive,
								AllowRenoteToExternal: ch.AllowRenoteToExternal,
							}
						}
					}
				}
			}
		}
	}
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
