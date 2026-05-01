package admin

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// PromoCreate handles POST /api/admin/promo/create.
func (h *Handler) PromoCreate(c echo.Context) error {
	if h.promoNoteRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		NoteID    string `json:"noteId"`
		ExpiresAt int64  `json:"expiresAt"`
	}
	if err := c.Bind(&req); err != nil || req.NoteID == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "noteId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}

	// 対象 note の存在確認 → 既に promote 済みでないか確認
	var targetUserID string
	if h.noteFinder != nil {
		note, err := h.noteFinder.FindByID(req.NoteID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, apierr.Error("NO_SUCH_NOTE", "No such note.", "ee449fbe-af2a-453b-9cae-cf2fe7c895fc"))
		}
		targetUserID = note.UserID
	}

	exists, err := h.promoNoteRepo.Exists(req.NoteID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	if exists {
		return c.JSON(http.StatusBadRequest, apierr.Error("ALREADY_PROMOTED", "The note has already promoted.", "ae427aa2-7a41-484f-a18c-2c1104051604"))
	}

	// note 情報が取れなかった場合は呼び出し admin を userId として記録 (最低限の整合)
	if targetUserID == "" {
		if u := middleware.GetUser(c); u != nil {
			targetUserID = u.ID
		}
	}

	promo := &model.PromoNote{
		NoteID:    req.NoteID,
		ExpiresAt: time.UnixMilli(req.ExpiresAt),
		UserID:    targetUserID,
	}
	if err := h.promoNoteRepo.Create(promo); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	return c.NoContent(http.StatusNoContent)
}
