package admin

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	"github.com/shiroha-a/mk/internal/model"
)

// SystemWebhookCreate handles POST /api/admin/system-webhook/create.
func (h *Handler) SystemWebhookCreate(c echo.Context) error {
	if h.systemWebhookRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		Name     string   `json:"name"`
		URL      string   `json:"url"`
		Secret   string   `json:"secret"`
		On       []string `json:"on"`
		IsActive *bool    `json:"isActive"`
	}
	if err := c.Bind(&req); err != nil || req.Name == "" || req.URL == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "Invalid parameters.", "00000000-0000-0000-0000-000000000000"))
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	sw := &model.SystemWebhook{
		ID:        h.idGen.Generate(time.Now()),
		Name:      req.Name,
		URL:       req.URL,
		Secret:    req.Secret,
		On:        req.On,
		IsActive:  isActive,
		UpdatedAt: time.Now(),
	}
	if err := h.systemWebhookRepo.Create(sw); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	return c.JSON(http.StatusOK, sw)
}

// SystemWebhookDelete handles POST /api/admin/system-webhook/delete.
func (h *Handler) SystemWebhookDelete(c echo.Context) error {
	if h.systemWebhookRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		ID string `json:"id"`
	}
	_ = c.Bind(&req)
	_ = h.systemWebhookRepo.Delete(req.ID)
	return c.NoContent(http.StatusNoContent)
}

// SystemWebhookList handles POST /api/admin/system-webhook/list.
func (h *Handler) SystemWebhookList(c echo.Context) error {
	if h.systemWebhookRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	rows, err := h.systemWebhookRepo.List()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	if rows == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	return c.JSON(http.StatusOK, rows)
}

// SystemWebhookShow handles POST /api/admin/system-webhook/show.
func (h *Handler) SystemWebhookShow(c echo.Context) error {
	if h.systemWebhookRepo == nil {
		return c.JSON(http.StatusNotFound, apierr.Error("NOT_FOUND", "Not found.", "00000000-0000-0000-0000-000000000000"))
	}
	var req struct {
		ID string `json:"id"`
	}
	_ = c.Bind(&req)
	sw, err := h.systemWebhookRepo.FindByID(req.ID)
	if err != nil {
		return c.JSON(http.StatusNotFound, apierr.Error("NOT_FOUND", "Not found.", "00000000-0000-0000-0000-000000000000"))
	}
	return c.JSON(http.StatusOK, sw)
}

// SystemWebhookTest handles POST /api/admin/system-webhook/test.
func (h *Handler) SystemWebhookTest(c echo.Context) error {
	var req struct {
		WebhookID string `json:"webhookId"`
		Type      string `json:"type"`
	}
	_ = c.Bind(&req)
	if req.WebhookID == "" || h.systemWebhookRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	sw, err := h.systemWebhookRepo.FindByID(req.WebhookID)
	if err != nil {
		return c.NoContent(http.StatusNoContent)
	}
	// テスト送信(非同期)。配送結果は latestStatus 系カラムに反映されないが、
	// Misskey 本家の /system-webhook/test も fire-and-forget 挙動なので整合。
	go sendWebhookTest(sw.URL, sw.Secret, req.Type)
	return c.NoContent(http.StatusNoContent)
}

// SystemWebhookUpdate handles POST /api/admin/system-webhook/update.
//
// 配送 processor が並行して latestSentAt/latestStatus を書き換えるため、
// FindByID→Save で全列上書きすると配送ステータスを古い値で踏み潰す。partial
// update (UpdateAdminFields) を使い admin 編集可能列のみ触る。
func (h *Handler) SystemWebhookUpdate(c echo.Context) error {
	if h.systemWebhookRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		ID       string    `json:"id"`
		Name     *string   `json:"name"`
		URL      *string   `json:"url"`
		Secret   *string   `json:"secret"`
		On       *[]string `json:"on"`
		IsActive *bool     `json:"isActive"`
	}
	if err := c.Bind(&req); err != nil || req.ID == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "Invalid parameters.", "00000000-0000-0000-0000-000000000000"))
	}
	// 存在確認 (GORM Updates(map) は 0 行影響でも nil を返すため)
	if _, err := h.systemWebhookRepo.FindByID(req.ID); err != nil {
		return c.JSON(http.StatusNotFound, apierr.Error("NOT_FOUND", "Not found.", "00000000-0000-0000-0000-000000000000"))
	}
	fields := map[string]any{"updatedAt": time.Now()}
	if req.Name != nil {
		fields["name"] = *req.Name
	}
	if req.URL != nil {
		fields["url"] = *req.URL
	}
	if req.Secret != nil {
		fields["secret"] = *req.Secret
	}
	if req.On != nil {
		fields["on"] = *req.On
	}
	if req.IsActive != nil {
		fields["isActive"] = *req.IsActive
	}
	if err := h.systemWebhookRepo.UpdateAdminFields(req.ID, fields); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	sw, err := h.systemWebhookRepo.FindByID(req.ID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	return c.JSON(http.StatusOK, sw)
}
