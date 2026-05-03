package admin

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	"github.com/shiroha-a/mk/internal/core/moderationlog"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// InviteCreate handles POST /api/admin/invite/create.
func (h *Handler) InviteCreate(c echo.Context) error {
	// 本家 TS は count (1-100, default 1) 分のチケットを配列で返す。個々の Create
	// 失敗時でも既作成分はロールバックしない (本家も Promise.all で非原子的)。
	if h.inviteRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		Count     int     `json:"count"`
		ExpiresAt *string `json:"expiresAt"`
	}
	_ = c.Bind(&req)
	if req.Count <= 0 {
		req.Count = 1
	}
	if req.Count > 100 {
		req.Count = 100
	}
	var expiresAt *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		parsed, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_DATE_TIME", "Invalid date-time format", "f1380b15-3760-4c6c-a1db-5c3aaf1cbd49"))
		}
		expiresAt = &parsed
	}
	user := middleware.GetUser(c)
	var createdByID *string
	if user != nil {
		createdByID = &user.ID
	}
	tickets := make([]*model.RegistrationTicket, 0, req.Count)
	for i := 0; i < req.Count; i++ {
		b := make([]byte, 16)
		_, _ = rand.Read(b)
		t := &model.RegistrationTicket{
			ID:          h.idGen.Generate(time.Now()),
			Code:        hex.EncodeToString(b),
			ExpiresAt:   expiresAt,
			CreatedByID: createdByID,
		}
		if err := h.inviteRepo.Create(t); err != nil {
			continue
		}
		tickets = append(tickets, t)
	}
	if len(tickets) > 0 {
		h.logModeration(c, moderationlog.LogCreateInvitation, map[string]any{
			"invitations": tickets,
		})
	}
	return c.JSON(http.StatusOK, h.packInviteTickets(tickets))
}

// packInviteTickets transforms RegistrationTicket rows into the
// Misskey-compatible InviteCodeEntityService.pack shape.
func (h *Handler) packInviteTickets(rows []*model.RegistrationTicket) []map[string]any {
	// Misskey 本家 InviteCodeEntityService.pack と同じ形にする。
	// createdAt は aidx ID から抽出、used は usedAt の有無で導出する。
	out := make([]map[string]any, 0, len(rows))
	for _, t := range rows {
		var createdAt *string
		if s, err := aidxCreatedAtString(h.idGen, t.ID); err == nil {
			createdAt = &s
		}
		var expiresAt *string
		if t.ExpiresAt != nil {
			s := t.ExpiresAt.UTC().Format("2006-01-02T15:04:05.000Z")
			expiresAt = &s
		}
		var usedAt *string
		if t.UsedAt != nil {
			s := t.UsedAt.UTC().Format("2006-01-02T15:04:05.000Z")
			usedAt = &s
		}
		out = append(out, map[string]any{
			"id":          t.ID,
			"code":        t.Code,
			"expiresAt":   expiresAt,
			"createdAt":   createdAt,
			"createdBy":   nil,
			"usedBy":      nil,
			"usedAt":      usedAt,
			"used":        t.UsedAt != nil,
			"createdById": t.CreatedByID,
			"usedById":    t.UsedByID,
		})
	}
	return out
}

// InviteList handles POST /api/admin/invite/list.
func (h *Handler) InviteList(c echo.Context) error {
	if h.inviteRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	var req struct {
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
		Type   string `json:"type"`
	}
	_ = c.Bind(&req)
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 30
	}
	filter := req.Type
	switch filter {
	case "unused", "used", "expired", "all":
	default:
		filter = "all"
	}
	rows, err := h.inviteRepo.List(filter, req.Limit, req.Offset, time.Now())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	return c.JSON(http.StatusOK, h.packInviteTickets(rows))
}
