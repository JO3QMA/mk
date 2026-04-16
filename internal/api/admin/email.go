package admin

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

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
