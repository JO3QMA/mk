package notes

import (
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
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
	noteRepo          repository.NoteRepository
	createService     *note.CreateService
	deleteService     *note.DeleteService
	queryService      *note.QueryService
	timelineService   *timeline.Service
	reactionService   *reaction.Service
	pollService       *poll.Service
	searchService     *search.Service
	idGen             id.Generator
	favoriteRepo      repository.NoteFavoriteRepository
	driveFileRepo     repository.DriveFileRepository
	draftRepo         repository.NoteDraftRepository
	noteReactionRepo  repository.NoteReactionRepository
	channelRepo       repository.ChannelRepository
	channelMutingRepo repository.ChannelMutingRepository
	instanceRepo      repository.InstanceRepository
	emojiRepo         repository.EmojiRepository
	driveFolderRepo   repository.DriveFolderRepository
	userRepo          repository.UserRepository
	userListRepo      repository.UserListRepository
	// ugcVisibility controls what unauthenticated visitors can see.
	// "all" (default), "local", "none"
	ugcVisibility string
	translator    *translate.DeepLClient
}

// SetChannelMutingRepo attaches a ChannelMutingRepository so timeline handlers
// can exclude notes posted to channels the viewer has muted.
func (h *Handler) SetChannelMutingRepo(r repository.ChannelMutingRepository) {
	h.channelMutingRepo = r
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

// SetInstanceRepo attaches an InstanceRepository so remote user embeds in
// note responses get their `instance` field populated (#277).
func (h *Handler) SetInstanceRepo(r repository.InstanceRepository) {
	h.instanceRepo = r
}

// SetEmojiRepo attaches an EmojiRepository so custom emoji shortcodes in
// note text and user displayNames get resolved to URLs (#330).
func (h *Handler) SetEmojiRepo(r repository.EmojiRepository) {
	h.emojiRepo = r
}

// SetDriveFolderRepo attaches a DriveFolderRepository so attached DriveFiles
// in note responses can embed the owning folder (#317).
func (h *Handler) SetDriveFolderRepo(r repository.DriveFolderRepository) {
	h.driveFolderRepo = r
}

// SetUserRepo attaches a UserRepository so attached DriveFiles in note
// responses can embed the owning user (#317).
func (h *Handler) SetUserRepo(r repository.UserRepository) {
	h.userRepo = r
}

// SetUserListRepo attaches a UserListRepository for user-list-timeline.
func (h *Handler) SetUserListRepo(r repository.UserListRepository) {
	h.userListRepo = r
}

// instanceLookup returns the repository as an entity.InstanceLookup, or nil
// when no repo has been wired. Narrowing to the entity interface keeps the
// packer independent from repository details.
func (h *Handler) instanceLookup() entity.InstanceLookup {
	if h.instanceRepo == nil {
		return nil
	}
	return h.instanceRepo
}

// emojiLookup returns the repository as an entity.EmojiLookup, or nil when
// no repo has been wired.
func (h *Handler) emojiLookup() entity.EmojiLookup {
	if h.emojiRepo == nil {
		return nil
	}
	return h.emojiRepo
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
		return apierr.JSONInvalidParam(c)
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
			return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "Text, fileIds, or renoteId is required.", apierr.UUIDInvalidParam))
		case errors.Is(err, note.ErrReplyTargetNotFound):
			return apierr.JSONNoSuchReplyTarget(c)
		case errors.Is(err, note.ErrRenoteTargetNotFound):
			return apierr.JSONNoSuchRenoteTarget(c)
		case errors.Is(err, note.ErrCannotReplyToInvisibleNote):
			return apierr.JSONCannotReplyToAnInvisibleNote(c)
		case errors.Is(err, note.ErrCannotRenoteInvisibleNote):
			return apierr.JSONCannotRenoteDueToVisibility(c)
		case errors.Is(err, note.ErrChannelNotFound):
			return apierr.JSONNoSuchChannel(c)
		case errors.Is(err, note.ErrCannotRenoteToAPureRenote):
			return apierr.JSONCannotRenoteToAPureRenote(c)
		case errors.Is(err, note.ErrCannotReplyToAPureRenote):
			return apierr.JSONCannotReplyToAPureRenote(c)
		case errors.Is(err, note.ErrCannotReplyToSpecifiedVisibility):
			return apierr.JSONCannotReplyToSpecifiedVisibilityNoteWithExtendedVisibility(c)
		case errors.Is(err, note.ErrYouHaveBeenBlocked):
			return apierr.JSONYouHaveBeenBlocked(c)
		case errors.Is(err, note.ErrCannotCreateAlreadyExpiredPoll):
			return apierr.JSONCannotCreateAlreadyExpiredPoll(c)
		case errors.Is(err, note.ErrNoSuchFile):
			return apierr.JSONNoSuchFile(c)
		case errors.Is(err, note.ErrCannotRenoteOutsideOfChannel):
			return apierr.JSONCannotRenoteOutsideOfChannel(c)
		case errors.Is(err, note.ErrContainsProhibitedWords):
			return apierr.JSONContainsProhibitedWords(c)
		case errors.Is(err, note.ErrContainsTooManyMentions):
			return apierr.JSONContainsTooManyMentions(c)
		}
		return apierr.JSONInternalError(c)
	}

	packed := entity.PackNoteWithInstance(created, h.idGen, h.instanceLookup(), h.emojiLookup())
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
		return apierr.JSONInvalidParam(c)
	}

	viewer := middleware.GetUser(c)
	n, err := h.lookupVisible(viewer, req.NoteID)
	if err != nil {
		return apierr.JSONNoSuchNote(c)
	}

	packed := entity.PackNoteWithInstance(n, h.idGen, h.instanceLookup(), h.emojiLookup())
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
		return apierr.JSONInvalidParam(c)
	}

	if err := h.deleteService.Delete(user, req.NoteID); err != nil {
		switch {
		case errors.Is(err, note.ErrNoteNotFound):
			return c.JSON(http.StatusNotFound, apierr.NoSuchNote())
		case errors.Is(err, note.ErrNoteAccessDenied):
			return c.JSON(http.StatusForbidden, apierr.Error("ACCESS_DENIED", "You are not the author of this note.", "fe8d7103-0ea8-4ec3-814d-f8b401dc69e9"))
		default:
			return apierr.JSONInternalError(c)
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
		return apierr.JSONInvalidParam(c)
	}
	req.normalize()

	viewer := middleware.GetUser(c)
	notes, err := fn(viewer, req)
	if err != nil {
		if errors.Is(err, note.ErrNoteNotFound) {
			return apierr.JSONNoSuchNote(c)
		}
		return apierr.JSONInternalError(c)
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
		return apierr.JSONInvalidParam(c)
	}
	if h.searchService == nil {
		return apierr.JSONInternalError(c)
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
			return apierr.JSONInvalidParam(c)
		}
		return apierr.JSONInternalError(c)
	}
	return c.JSON(http.StatusOK, h.packMany(notes, viewer))
}

// State handles POST /api/notes/state.
func (h *Handler) State(c echo.Context) error {
	var req ShowRequest
	if err := c.Bind(&req); err != nil || req.NoteID == "" {
		return apierr.JSONInvalidParam(c)
	}
	viewer := middleware.GetUser(c)
	st, err := h.queryService.State(viewer, req.NoteID)
	if err != nil {
		// 現状QueryService.StateはErrNoteNotFound以外を返さない
		return apierr.JSONNoSuchNote(c)
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
		return apierr.JSONInvalidParam(c)
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
		return apierr.JSONNoSuchNote(c)
	}
	return c.JSON(http.StatusOK, h.packMany(notes, viewer))
}

// BulkShow handles POST /api/notes — bulk note lookup by noteIds.
// visibility チェックを通して非公開ノートの漏洩を防ぐ。
func (h *Handler) BulkShow(c echo.Context) error {
	var req struct {
		NoteIDs []string `json:"noteIds"`
	}
	if err := c.Bind(&req); err != nil {
		return apierr.JSONInvalidParam(c)
	}
	if len(req.NoteIDs) == 0 {
		return c.JSON(http.StatusOK, []any{})
	}
	if len(req.NoteIDs) > 100 {
		req.NoteIDs = req.NoteIDs[:100]
	}
	notes, err := h.noteRepo.FindManyByIDsWithUser(req.NoteIDs)
	if err != nil {
		return c.JSON(http.StatusOK, []any{})
	}
	viewer := middleware.GetUser(c)
	if h.queryService != nil {
		notes = h.queryService.FilterVisible(viewer, notes)
	}
	return c.JSON(http.StatusOK, h.packMany(notes, viewer))
}

// packMany serializes a list of notes into NoteEntity objects.
// driveFileRepoが設定されている場合、ファイル情報を解決してFilesに含める。
// viewerがnon-nilの場合、myReactionなどのviewer依存フィールドも解決する。
func (h *Handler) packMany(notes []*model.Note, viewer *model.User) []entity.NoteEntity {
	out := entity.PackNotes(notes, h.idGen, h.instanceLookup(), h.emojiLookup())
	h.resolveFiles(out)
	h.resolveViewerFields(out, viewer)
	return out
}

// resolveFiles collects all fileIds from notes, fetches DriveFiles in bulk,
// and populates the Files field of each NoteEntity.
// Renote / Reply の embed エンティティにも同じ resolution を適用して、quote
// renote の添付が空になる問題を避ける (#416 関連)。
func (h *Handler) resolveFiles(notes []entity.NoteEntity) {
	if h.driveFileRepo == nil {
		return
	}
	// 全fileIDsを収集 (renote/reply の embed 分も含む)
	var allIDs []string
	for i := range notes {
		allIDs = appendNoteFileIDs(allIDs, &notes[i])
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
	// folder / user を best-effort で pre-fetch して embed する (#317)。
	// 添付ファイル経由で重複して参照される folder/user を最小限の read に
	// 抑えるため、unique ID を集めて cache に流し込む。N+1 は残るが attachment
	// 数は実用上少ないので許容 (batch 最適化は follow-up)。
	folderCache, userCache := h.resolveFileOwners(files)

	for i := range notes {
		h.populateNoteFiles(&notes[i], fileMap, folderCache, userCache)
	}
}

// appendNoteFileIDs collects fileIDs from the note plus its embedded Renote /
// Reply targets (1 level deep, matching the repository preload depth).
func appendNoteFileIDs(dst []string, n *entity.NoteEntity) []string {
	if n == nil {
		return dst
	}
	dst = append(dst, n.FileIDs...)
	if n.Renote != nil {
		dst = append(dst, n.Renote.FileIDs...)
	}
	if n.Reply != nil {
		dst = append(dst, n.Reply.FileIDs...)
	}
	return dst
}

// populateNoteFiles fills NoteEntity.Files for the note plus its embedded
// Renote / Reply targets using the shared file / folder / user caches.
func (h *Handler) populateNoteFiles(n *entity.NoteEntity, fileMap map[string]*model.DriveFile, folderCache map[string]*model.DriveFolder, userCache map[string]*model.User) {
	if n == nil {
		return
	}
	n.Files = h.packNoteFiles(n.FileIDs, fileMap, folderCache, userCache)
	if n.Renote != nil {
		n.Renote.Files = h.packNoteFiles(n.Renote.FileIDs, fileMap, folderCache, userCache)
	}
	if n.Reply != nil {
		n.Reply.Files = h.packNoteFiles(n.Reply.FileIDs, fileMap, folderCache, userCache)
	}
}

// packNoteFiles resolves a note's fileIDs into packed file entities using the
// shared cache populated by resolveFiles.
func (h *Handler) packNoteFiles(fileIDs []string, fileMap map[string]*model.DriveFile, folderCache map[string]*model.DriveFolder, userCache map[string]*model.User) []any {
	packed := make([]any, 0, len(fileIDs))
	for _, fid := range fileIDs {
		f, ok := fileMap[fid]
		if !ok {
			continue
		}
		var folder *model.DriveFolder
		if f.FolderID != nil {
			folder = folderCache[*f.FolderID]
		}
		var user *model.User
		if f.UserID != nil {
			user = userCache[*f.UserID]
		}
		packed = append(packed, entity.PackDriveFileWithRelations(f, h.idGen, folder, user))
	}
	return packed
}

// resolveFileOwners builds id→model caches for folder / user by iterating
// the unique IDs referenced by files. Missing repos / lookup failures are
// silently treated as "no embed" (PackDriveFileWithRelations handles nil).
func (h *Handler) resolveFileOwners(files []*model.DriveFile) (map[string]*model.DriveFolder, map[string]*model.User) {
	folderCache := map[string]*model.DriveFolder{}
	userCache := map[string]*model.User{}
	for _, f := range files {
		if h.driveFolderRepo != nil && f.FolderID != nil {
			if _, seen := folderCache[*f.FolderID]; !seen {
				if folder, err := h.driveFolderRepo.FindByID(*f.FolderID); err == nil {
					folderCache[*f.FolderID] = folder
				} else {
					folderCache[*f.FolderID] = nil
				}
			}
		}
		if h.userRepo != nil && f.UserID != nil {
			if _, seen := userCache[*f.UserID]; !seen {
				if u, err := h.userRepo.FindByID(*f.UserID); err == nil {
					userCache[*f.UserID] = u
				} else {
					userCache[*f.UserID] = nil
				}
			}
		}
	}
	return folderCache, userCache
}

// resolveViewerFields populates viewer-dependent fields (MyReaction, Channel)
// on packed NoteEntities. viewerがnilの場合はChannel解決のみ行う。
// Renote / Reply の embed にも同じ処理を適用する (#416)。
func (h *Handler) resolveViewerFields(notes []entity.NoteEntity, viewer *model.User) {
	if len(notes) == 0 {
		return
	}

	// MyReaction: viewerが認証済みの場合にバッチ取得 (embed 分も同じクエリで)
	if viewer != nil && h.noteReactionRepo != nil {
		var noteIDs []string
		for i := range notes {
			noteIDs = appendNoteIDs(noteIDs, &notes[i])
		}
		reactionMap, err := h.noteReactionRepo.FindByUserAndNoteIDs(viewer.ID, noteIDs)
		if err == nil {
			for i := range notes {
				applyMyReaction(&notes[i], reactionMap)
			}
		}
	}

	// Channel: ChannelIDがnon-nilのノートについてバッチ取得 (embed 分も含む)
	if h.channelRepo != nil {
		var channelIDs []string
		for i := range notes {
			channelIDs = appendNoteChannelIDs(channelIDs, &notes[i])
		}
		if len(channelIDs) > 0 {
			channels, err := h.channelRepo.FindByIDs(channelIDs)
			if err == nil {
				chMap := make(map[string]*model.Channel, len(channels))
				for _, ch := range channels {
					chMap[ch.ID] = ch
				}
				for i := range notes {
					applyChannel(&notes[i], chMap)
				}
			}
		}
	}
}

// appendNoteIDs collects the id of the note plus embedded Renote / Reply.
func appendNoteIDs(dst []string, n *entity.NoteEntity) []string {
	if n == nil {
		return dst
	}
	dst = append(dst, n.ID)
	if n.Renote != nil {
		dst = append(dst, n.Renote.ID)
	}
	if n.Reply != nil {
		dst = append(dst, n.Reply.ID)
	}
	return dst
}

// applyMyReaction fills MyReaction on the note and embedded renote/reply.
func applyMyReaction(n *entity.NoteEntity, reactionMap map[string]*model.NoteReaction) {
	if n == nil {
		return
	}
	if r, ok := reactionMap[n.ID]; ok {
		nr := entity.NormalizeReactionKey(r.Reaction)
		n.MyReaction = &nr
	}
	if n.Renote != nil {
		if r, ok := reactionMap[n.Renote.ID]; ok {
			nr := entity.NormalizeReactionKey(r.Reaction)
			n.Renote.MyReaction = &nr
		}
	}
	if n.Reply != nil {
		if r, ok := reactionMap[n.Reply.ID]; ok {
			nr := entity.NormalizeReactionKey(r.Reaction)
			n.Reply.MyReaction = &nr
		}
	}
}

// appendNoteChannelIDs collects channelIDs from the note plus embedded
// Renote / Reply.
func appendNoteChannelIDs(dst []string, n *entity.NoteEntity) []string {
	if n == nil {
		return dst
	}
	if n.ChannelID != nil && *n.ChannelID != "" {
		dst = append(dst, *n.ChannelID)
	}
	if n.Renote != nil && n.Renote.ChannelID != nil && *n.Renote.ChannelID != "" {
		dst = append(dst, *n.Renote.ChannelID)
	}
	if n.Reply != nil && n.Reply.ChannelID != nil && *n.Reply.ChannelID != "" {
		dst = append(dst, *n.Reply.ChannelID)
	}
	return dst
}

// applyChannel fills Channel on the note and embedded renote/reply.
func applyChannel(n *entity.NoteEntity, chMap map[string]*model.Channel) {
	if n == nil {
		return
	}
	setChannel := func(target *entity.NoteEntity) {
		if target == nil || target.ChannelID == nil {
			return
		}
		ch, ok := chMap[*target.ChannelID]
		if !ok {
			return
		}
		target.Channel = &entity.ChannelLite{
			ID:                    ch.ID,
			Name:                  ch.Name,
			Color:                 ch.Color,
			IsSensitive:           ch.IsSensitive,
			AllowRenoteToExternal: ch.AllowRenoteToExternal,
		}
	}
	setChannel(n)
	setChannel(n.Renote)
	setChannel(n.Reply)
}
