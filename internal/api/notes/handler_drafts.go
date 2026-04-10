package notes

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/server/middleware"
	"gorm.io/gorm"
)

// draftRepo provides note_draft CRUD via GORM.
// ハンドラが直接DBを持つのではなく、SetDBで後付けする。
type draftDB struct {
	db *gorm.DB
}

// SetDraftDB attaches a DB connection for draft operations.
func (h *Handler) SetDraftDB(db *gorm.DB) {
	h.draftDB = db
}

// DraftsList handles POST /api/notes/drafts/list.
func (h *Handler) DraftsList(c echo.Context) error {
	user := middleware.GetUser(c)
	if h.draftDB == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	var drafts []*model.NoteDraft
	h.draftDB.Where(`"userId" = ?`, user.ID).Order(`"id" DESC`).Limit(20).Find(&drafts)
	out := make([]map[string]any, len(drafts))
	for i, d := range drafts {
		out[i] = packDraft(d, h.idGen)
	}
	return c.JSON(http.StatusOK, out)
}

// DraftsCreate handles POST /api/notes/drafts/create.
func (h *Handler) DraftsCreate(c echo.Context) error {
	user := middleware.GetUser(c)
	if h.draftDB == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		Text       *string  `json:"text"`
		CW         *string  `json:"cw"`
		Visibility string   `json:"visibility"`
		FileIDs    []string `json:"fileIds"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "Invalid parameters.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if req.Visibility == "" {
		req.Visibility = "public"
	}
	draft := &model.NoteDraft{
		ID:         h.idGen.Generate(time.Now()),
		UserID:     user.ID,
		Text:       req.Text,
		CW:         req.CW,
		Visibility: req.Visibility,
		FileIDs:    req.FileIDs,
	}
	if err := h.draftDB.Create(draft).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, apiError("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.JSON(http.StatusOK, packDraft(draft, h.idGen))
}

// DraftsUpdate handles POST /api/notes/drafts/update.
func (h *Handler) DraftsUpdate(c echo.Context) error {
	user := middleware.GetUser(c)
	if h.draftDB == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		DraftID    string   `json:"draftId"`
		Text       *string  `json:"text"`
		CW         *string  `json:"cw"`
		Visibility string   `json:"visibility"`
		FileIDs    []string `json:"fileIds"`
	}
	if err := c.Bind(&req); err != nil || req.DraftID == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "draftId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	var draft model.NoteDraft
	if err := h.draftDB.Where(`"id" = ? AND "userId" = ?`, req.DraftID, user.ID).First(&draft).Error; err != nil {
		return c.JSON(http.StatusNotFound, apiError("NO_SUCH_DRAFT", "No such draft.", "00000000-0000-0000-0000-000000000000"))
	}
	if req.Text != nil {
		draft.Text = req.Text
	}
	if req.CW != nil {
		draft.CW = req.CW
	}
	if req.Visibility != "" {
		draft.Visibility = req.Visibility
	}
	if req.FileIDs != nil {
		draft.FileIDs = req.FileIDs
	}
	h.draftDB.Save(&draft)
	return c.JSON(http.StatusOK, packDraft(&draft, h.idGen))
}

// DraftsDelete handles POST /api/notes/drafts/delete.
func (h *Handler) DraftsDelete(c echo.Context) error {
	user := middleware.GetUser(c)
	if h.draftDB == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		DraftID string `json:"draftId"`
	}
	if err := c.Bind(&req); err != nil || req.DraftID == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "draftId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	h.draftDB.Where(`"id" = ? AND "userId" = ?`, req.DraftID, user.ID).Delete(&model.NoteDraft{})
	return c.NoContent(http.StatusNoContent)
}

// DraftsCount handles POST /api/notes/drafts/count.
func (h *Handler) DraftsCount(c echo.Context) error {
	user := middleware.GetUser(c)
	if h.draftDB == nil {
		return c.JSON(http.StatusOK, map[string]any{"count": 0})
	}
	var count int64
	h.draftDB.Model(&model.NoteDraft{}).Where(`"userId" = ?`, user.ID).Count(&count)
	return c.JSON(http.StatusOK, map[string]any{"count": count})
}

// ThreadMutingCreate handles POST /api/notes/thread-muting/create.
func (h *Handler) ThreadMutingCreate(c echo.Context) error {
	var req struct {
		NoteID string `json:"noteId"`
	}
	if err := c.Bind(&req); err != nil || req.NoteID == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "noteId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	// スレッドミュート: noteIDをthreadIDとして使用 (簡易実装)
	return c.NoContent(http.StatusNoContent)
}

// ThreadMutingDelete handles POST /api/notes/thread-muting/delete.
func (h *Handler) ThreadMutingDelete(c echo.Context) error {
	var req struct {
		NoteID string `json:"noteId"`
	}
	if err := c.Bind(&req); err != nil || req.NoteID == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "noteId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	return c.NoContent(http.StatusNoContent)
}

// PollsRecommendation handles POST /api/notes/polls/recommendation.
func (h *Handler) PollsRecommendation(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

func packDraft(d *model.NoteDraft, idGen interface {
	ParseTime(string) (time.Time, error)
}) map[string]any {
	result := map[string]any{
		"id":         d.ID,
		"userId":     d.UserID,
		"text":       d.Text,
		"cw":         d.CW,
		"visibility": d.Visibility,
		"localOnly":  d.LocalOnly,
		"fileIds":    d.FileIDs,
	}
	if t, err := idGen.ParseTime(d.ID); err == nil {
		result["createdAt"] = t.UTC().Format("2006-01-02T15:04:05.000Z")
	}
	return result
}
