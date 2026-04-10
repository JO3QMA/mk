package admin

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/smtp"
	"strings"
	"time"
)

// sendEmailSMTP sends an email via SMTP. ベストエフォート (goroutineで呼ばれる)。
func sendEmailSMTP(host string, port int, user, pass *string, from, to, subject, body string) {
	addr := fmt.Sprintf("%s:%d", host, port)

	var auth smtp.Auth
	if user != nil && pass != nil && *user != "" {
		auth = smtp.PlainAuth("", *user, *pass, host)
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		from, to, subject, body)

	// TLS対応
	tlsConfig := &tls.Config{ServerName: host}
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		slog.Warn("email: dial failed", "error", err)
		return
	}

	c, err := smtp.NewClient(conn, host)
	if err != nil {
		slog.Warn("email: smtp client failed", "error", err)
		return
	}
	defer c.Close()

	if err := c.StartTLS(tlsConfig); err != nil {
		// TLS非対応の場合は無視して続行
		slog.Debug("email: STARTTLS not supported", "error", err)
	}

	if auth != nil {
		if err := c.Auth(auth); err != nil {
			slog.Warn("email: auth failed", "error", err)
			return
		}
	}

	if err := c.Mail(from); err != nil {
		slog.Warn("email: MAIL FROM failed", "error", err)
		return
	}
	if err := c.Rcpt(to); err != nil {
		slog.Warn("email: RCPT TO failed", "error", err)
		return
	}

	w, err := c.Data()
	if err != nil {
		slog.Warn("email: DATA failed", "error", err)
		return
	}
	_, _ = w.Write([]byte(msg))
	_ = w.Close()
	_ = c.Quit()

	slog.Info("email sent", "to", to, "subject", subject)
}

// sendWebhookTest sends a test webhook POST request.
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

// init suppresses unused import warnings
