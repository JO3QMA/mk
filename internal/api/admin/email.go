package admin

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/misc/smtp"
)

// SendEmail handles POST /api/admin/send-email.
func (h *Handler) SendEmail(c echo.Context) error {
	var req struct {
		To      string `json:"to"`
		Subject string `json:"subject"`
		Text    string `json:"text"`
	}
	if err := c.Bind(&req); err != nil || req.To == "" {
		return c.NoContent(http.StatusNoContent)
	}
	// SMTP送信
	if h.metaRepo != nil {
		m, err := h.metaRepo.Fetch()
		if err == nil && m.EnableEmail && m.SmtpHost != nil && m.Email != nil {
			port := 587
			if m.SmtpPort != nil {
				port = *m.SmtpPort
			}
			go smtp.SendWithOptions(*m.SmtpHost, port, m.SmtpUser, m.SmtpPass, *m.Email, req.To, req.Subject, req.Text, smtp.Options{ProxyURL: h.smtpProxyURL})
		}
	}
	return c.NoContent(http.StatusNoContent)
}

// sendWebhookTest sends a test webhook POST request.
//
// Misskey 本家の admin/system-webhook/test 互換。シークレットがあれば
// X-Misskey-Hook-Secret ヘッダに平文を載せる (本家も同仕様)。
func sendWebhookTest(url, secret, eventType string) {
	body := fmt.Sprintf(`{"type":"%s","body":{"test":true},"createdAt":"%s"}`,
		eventType, time.Now().UTC().Format(time.RFC3339))

	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		slog.Warn("webhook test: request creation failed", "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		req.Header.Set("X-Misskey-Hook-Secret", secret)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		slog.Warn("webhook test: request failed", "error", err)
		return
	}
	_ = resp.Body.Close()
	slog.Info("webhook test sent", "url", url, "status", resp.StatusCode)
}
