package webhooks

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// TestDispatcher is the minimal interface the Test endpoint uses to enqueue
// a synthetic webhook payload. 循環依存を避けるため interface で受ける
// (実装は core/webhook.Service 経由、dispatch through DispatchUser)。
type TestDispatcher interface {
	DispatchUser(userID, eventType string, body any)
}

// Handler handles i/webhooks/* endpoints.
type Handler struct {
	repo       repository.WebhookRepository
	idGen      id.Generator
	dispatcher TestDispatcher
}

// NewHandler creates a new webhooks handler.
func NewHandler(repo repository.WebhookRepository, idGen id.Generator) *Handler {
	return &Handler{repo: repo, idGen: idGen}
}

// SetDispatcher wires a TestDispatcher so that /api/i/webhooks/test can fire
// a synthetic test payload through the production pipeline.
func (h *Handler) SetDispatcher(d TestDispatcher) {
	h.dispatcher = d
}

func apiError(code, message, errID string) map[string]any {
	return apierr.Error(code, message, errID)
}

func packWebhook(w *model.Webhook) map[string]any {
	return map[string]any{
		"id":           w.ID,
		"userId":       w.UserID,
		"name":         w.Name,
		"on":           w.On,
		"url":          w.URL,
		"secret":       w.Secret,
		"active":       w.Active,
		"latestSentAt": w.LatestSentAt,
		"latestStatus": w.LatestStatus,
	}
}

// Create handles POST /api/i/webhooks/create.
func (h *Handler) Create(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		Name   string   `json:"name"`
		URL    string   `json:"url"`
		Secret string   `json:"secret"`
		On     []string `json:"on"`
	}
	if err := c.Bind(&req); err != nil || req.Name == "" || req.URL == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "name and url are required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}

	webhook := &model.Webhook{
		ID:     h.idGen.Generate(time.Now()),
		UserID: user.ID,
		Name:   req.Name,
		URL:    req.URL,
		Secret: req.Secret,
		On:     req.On,
		Active: true,
	}
	if err := h.repo.Create(webhook); err != nil {
		return c.JSON(http.StatusInternalServerError, apiError("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}

	return c.JSON(http.StatusOK, packWebhook(webhook))
}

// List handles POST /api/i/webhooks/list.
func (h *Handler) List(c echo.Context) error {
	user := middleware.GetUser(c)
	webhooks, err := h.repo.ListByUserID(user.ID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apiError("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	result := make([]map[string]any, len(webhooks))
	for i, w := range webhooks {
		result[i] = packWebhook(w)
	}
	return c.JSON(http.StatusOK, result)
}

// Show handles POST /api/i/webhooks/show.
func (h *Handler) Show(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		WebhookID string `json:"webhookId"`
	}
	if err := c.Bind(&req); err != nil || req.WebhookID == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "webhookId is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}

	w, err := h.repo.FindByIDAndUserID(req.WebhookID, user.ID)
	if err != nil {
		return c.JSON(http.StatusNotFound, apiError("NO_SUCH_WEBHOOK", "No such webhook.", "50f614d9-3e73-4e43-8345-2e1e25012b7a"))
	}

	return c.JSON(http.StatusOK, packWebhook(w))
}

// Update handles POST /api/i/webhooks/update.
func (h *Handler) Update(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		WebhookID string   `json:"webhookId"`
		Name      string   `json:"name"`
		URL       string   `json:"url"`
		Secret    *string  `json:"secret"`
		On        []string `json:"on"`
		Active    *bool    `json:"active"`
	}
	if err := c.Bind(&req); err != nil || req.WebhookID == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "webhookId is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}

	w, err := h.repo.FindByIDAndUserID(req.WebhookID, user.ID)
	if err != nil {
		return c.JSON(http.StatusNotFound, apiError("NO_SUCH_WEBHOOK", "No such webhook.", "50f614d9-3e73-4e43-8345-2e1e25012b7a"))
	}

	if req.Name != "" {
		w.Name = req.Name
	}
	if req.URL != "" {
		w.URL = req.URL
	}
	if req.Secret != nil {
		w.Secret = *req.Secret
	}
	if req.On != nil {
		w.On = req.On
	}
	if req.Active != nil {
		w.Active = *req.Active
	}

	if err := h.repo.Update(w); err != nil {
		return c.JSON(http.StatusInternalServerError, apiError("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}

	return c.JSON(http.StatusOK, packWebhook(w))
}

// Delete handles POST /api/i/webhooks/delete.
func (h *Handler) Delete(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		WebhookID string `json:"webhookId"`
	}
	if err := c.Bind(&req); err != nil || req.WebhookID == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "webhookId is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}

	if _, err := h.repo.FindByIDAndUserID(req.WebhookID, user.ID); err != nil {
		return c.JSON(http.StatusNotFound, apiError("NO_SUCH_WEBHOOK", "No such webhook.", "50f614d9-3e73-4e43-8345-2e1e25012b7a"))
	}

	if err := h.repo.Delete(req.WebhookID, user.ID); err != nil {
		return c.JSON(http.StatusInternalServerError, apiError("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}

	return c.NoContent(http.StatusNoContent)
}

// Test handles POST /api/i/webhooks/test.
// 本家互換: req.type で指定されたイベント種別のテストペイロードを dispatcher
// に渡し、通常の配信パイプラインを通して登録済み webhook に送信する。
func (h *Handler) Test(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		WebhookID string `json:"webhookId"`
		Type      string `json:"type"`
	}
	if err := c.Bind(&req); err != nil || req.WebhookID == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "webhookId is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}

	webhook, err := h.repo.FindByIDAndUserID(req.WebhookID, user.ID)
	if err != nil {
		return c.JSON(http.StatusNotFound, apiError("NO_SUCH_WEBHOOK", "No such webhook.", "50f614d9-3e73-4e43-8345-2e1e25012b7a"))
	}

	if h.dispatcher != nil {
		eventType := req.Type
		if eventType == "" {
			eventType = "note"
		}
		// ダミー body。クライアント側で判定するための最低限のフィールドを入れる。
		h.dispatcher.DispatchUser(user.ID, eventType, map[string]any{
			"test":      true,
			"webhookId": webhook.ID,
		})
	}

	return c.NoContent(http.StatusNoContent)
}
