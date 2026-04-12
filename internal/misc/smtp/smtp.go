// Package smtp provides a best-effort SMTP email sender shared by admin
// and i handler packages. SMTP 設定エラーはログに出力し、呼び出し元にはエラーを
// 返さない (ベストエフォート送信)。
package smtp

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	gosmtp "net/smtp"
	"time"
)

// Send sends a plain-text email via SMTP.
func Send(host string, port int, user, pass *string, from, to, subject, body string) {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))

	var auth gosmtp.Auth
	if user != nil && pass != nil && *user != "" {
		auth = gosmtp.PlainAuth("", *user, *pass, host)
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		from, to, subject, body)

	tlsConfig := &tls.Config{ServerName: host}
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		slog.Warn("email: dial failed", "error", err)
		return
	}

	c, err := gosmtp.NewClient(conn, host)
	if err != nil {
		slog.Warn("email: smtp client failed", "error", err)
		return
	}
	defer c.Close()

	if err := c.StartTLS(tlsConfig); err != nil {
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
