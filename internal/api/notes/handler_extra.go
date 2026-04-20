package notes

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// SetFavoriteRepo attaches a NoteFavoriteRepository for favorites endpoints.
func (h *Handler) SetFavoriteRepo(r repository.NoteFavoriteRepository) {
	h.favoriteRepo = r
}

// FavoritesCreate handles POST /api/notes/favorites/create.
func (h *Handler) FavoritesCreate(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		NoteID string `json:"noteId"`
	}
	if err := c.Bind(&req); err != nil || req.NoteID == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "noteId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if _, err := h.noteRepo.FindByID(req.NoteID); err != nil {
		return apierr.JSONNoSuchNote(c)
	}
	if h.favoriteRepo == nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	exists, _ := h.favoriteRepo.Exists(user.ID, req.NoteID)
	if exists {
		return c.JSON(http.StatusConflict, apierr.Error("ALREADY_FAVORITED", "Already favorited.", "a402c12b-34dd-41d2-97d8-4d2c5b7e4645"))
	}
	fav := &model.NoteFavorite{
		ID:     h.idGen.Generate(time.Now()),
		UserID: user.ID,
		NoteID: req.NoteID,
	}
	if err := h.favoriteRepo.Create(fav); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.NoContent(http.StatusNoContent)
}

// FavoritesDelete handles POST /api/notes/favorites/delete.
func (h *Handler) FavoritesDelete(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		NoteID string `json:"noteId"`
	}
	if err := c.Bind(&req); err != nil || req.NoteID == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "noteId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if h.favoriteRepo == nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	if err := h.favoriteRepo.Delete(user.ID, req.NoteID); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.NoContent(http.StatusNoContent)
}

// Featured handles POST /api/notes/featured.
func (h *Handler) Featured(c echo.Context) error {
	// 注目ノート — renoteCount + repliesCount が高いノートを返す簡易版
	var req struct {
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "Invalid parameters.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}
	notes, err := h.noteRepo.ListFeatured(req.Limit, req.Offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.JSON(http.StatusOK, h.packMany(notes, middleware.GetUser(c)))
}

// Unrenote handles POST /api/notes/unrenote.
func (h *Handler) Unrenote(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		NoteID string `json:"noteId"`
	}
	if err := c.Bind(&req); err != nil || req.NoteID == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "noteId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	// renoteId が指定ノートの自分のノートを探して削除
	renote, err := h.noteRepo.FindRenoteByUser(user.ID, req.NoteID)
	if err != nil {
		return apierr.JSONNoSuchNote(c)
	}
	if err := h.deleteService.Delete(user, renote.ID); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.NoContent(http.StatusNoContent)
}

// Mentions handles POST /api/notes/mentions.
func (h *Handler) Mentions(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		Limit      int    `json:"limit"`
		SinceID    string `json:"sinceId"`
		UntilID    string `json:"untilId"`
		Visibility string `json:"visibility"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "Invalid parameters.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}
	notes, err := h.noteRepo.ListMentions(user.ID, req.Limit, req.SinceID, req.UntilID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	// visibilityフィルタ
	// "specified" → DMのみ、未指定 → DM以外
	var filtered []*model.Note
	for _, n := range notes {
		if req.Visibility == "specified" {
			if n.Visibility == model.NoteVisibilitySpecified {
				filtered = append(filtered, n)
			}
		} else {
			if n.Visibility != model.NoteVisibilitySpecified {
				filtered = append(filtered, n)
			}
		}
	}
	return c.JSON(http.StatusOK, h.packMany(filtered, user))
}

// UserListTimeline handles POST /api/notes/user-list-timeline.
func (h *Handler) UserListTimeline(c echo.Context) error {
	me := middleware.GetUser(c)
	var req struct {
		ListID  string `json:"listId"`
		Limit   int    `json:"limit"`
		SinceID string `json:"sinceId"`
		UntilID string `json:"untilId"`
	}
	if err := c.Bind(&req); err != nil || req.ListID == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "listId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Limit > 100 {
		req.Limit = 100
	}
	// リスト所有権チェック (TS互換: 自分のリストのみ閲覧可)
	if h.userListRepo != nil {
		list, err := h.userListRepo.FindByID(req.ListID)
		if err != nil {
			return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_LIST", "No such list.", "7bc05c21-1d7a-41ae-88f1-d8571571e198"))
		}
		if list.UserID != me.ID {
			return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_LIST", "No such list.", "7bc05c21-1d7a-41ae-88f1-d8571571e198"))
		}
	}
	notes, err := h.noteRepo.ListByUserList(req.ListID, req.Limit, req.SinceID, req.UntilID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.JSON(http.StatusOK, h.packMany(notes, me))
}

// SearchByTag handles POST /api/notes/search-by-tag.
// TS互換: tag (string) または query (string[][] — AND/OR組み合わせ) を受け付ける。
// query の完全なAND/OR交差はサポートせず、最初に見つかったタグで検索する。
func (h *Handler) SearchByTag(c echo.Context) error {
	var req struct {
		Tag     string     `json:"tag"`
		Query   [][]string `json:"query"`
		Limit   int        `json:"limit"`
		SinceID string     `json:"sinceId"`
		UntilID string     `json:"untilId"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "tag is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	// query 配列から tag を抽出 (tag が空の場合のフォールバック)
	if req.Tag == "" && len(req.Query) > 0 {
		for _, inner := range req.Query {
			if len(inner) > 0 && inner[0] != "" {
				req.Tag = inner[0]
				break
			}
		}
	}
	if req.Tag == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "tag is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}
	// tagsカラムにtagを含むノートを検索
	notes, err := h.noteRepo.SearchByTag(req.Tag, req.Limit, req.SinceID, req.UntilID)
	if err != nil {
		return c.JSON(http.StatusOK, []entity.NoteEntity{})
	}
	return c.JSON(http.StatusOK, h.packMany(notes, middleware.GetUser(c)))
}

// Clips handles POST /api/notes/clips.
func (h *Handler) Clips(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

// Translate handles POST /api/notes/translate.
func (h *Handler) Translate(c echo.Context) error {
	var req struct {
		NoteID     string `json:"noteId"`
		TargetLang string `json:"targetLang"`
	}
	if err := c.Bind(&req); err != nil || req.NoteID == "" || req.TargetLang == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "noteId and targetLang are required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}

	if h.translator == nil {
		return c.JSON(http.StatusServiceUnavailable, apierr.Error("UNAVAILABLE", "Translator is not configured.", "bef6e895-c05f-4572-96ab-58f5ae1e2e28"))
	}

	n, err := h.noteRepo.FindByID(req.NoteID)
	if err != nil {
		return apierr.JSONNoSuchNote(c)
	}

	if n.Text == nil || *n.Text == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("CANNOT_TRANSLATE", "Nothing to translate.", "bef6e895-c05f-4572-96ab-58f5ae1e2e28"))
	}

	result, err := h.translator.Translate(c.Request().Context(), *n.Text, req.TargetLang)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Translation failed.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}

	return c.JSON(http.StatusOK, map[string]any{
		"sourceLang": result.SourceLang,
		"text":       result.Text,
	})
}

// ShowPartialBulk handles POST /api/notes/show-partial-bulk.
func (h *Handler) ShowPartialBulk(c echo.Context) error {
	var req struct {
		NoteIDs []string `json:"noteIds"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "noteIds is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if len(req.NoteIDs) == 0 {
		return c.JSON(http.StatusOK, []entity.NoteEntity{})
	}
	notes, err := h.noteRepo.FindManyByIDsWithUser(req.NoteIDs)
	if err != nil {
		return c.JSON(http.StatusOK, []entity.NoteEntity{})
	}
	return c.JSON(http.StatusOK, h.packMany(notes, middleware.GetUser(c)))
}
