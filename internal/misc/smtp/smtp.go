// Package smtp provides a best-effort SMTP email sender shared by admin
// and i handler packages.
//
// SMTP設定エラーはログに出力し、呼び出し元にはエラーを返さない
// (ベストエフォート送信)。
package smtp

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net"
	gosmtp "net/smtp"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

// Options holds optional configuration for Send. Future flags (TLS mode,
// timeouts, etc) live here. Zero-value is "no overrides".
type Options struct {
	// ProxyURL routes the SMTP TCP connection through a forward proxy.
	// 形式は upstream Misskey の `proxySmtp` と同じく URL (scheme で
	// プロトコルを指定): `socks5://user:pass@host:1080` または
	// `http://host:3128` (HTTP CONNECT)。空文字なら direct dial。
	ProxyURL string
}

// sanitizeHeaderValue strips CR and LF characters to prevent SMTP header
// injection.
func sanitizeHeaderValue(s string) string {
	// 攻撃者がヘッダーフィールドに改行(CR/LF)を仕込むとBCC等の任意ヘッダーを
	// 注入できてしまうため、ここで無害化する。
	r := strings.NewReplacer("\r", "", "\n", "")
	return r.Replace(s)
}

// Send sends a plain-text email via SMTP. Direct dial; equivalent to
// SendWithOptions(.., Options{}).
func Send(host string, port int, user, pass *string, from, to, subject, body string) {
	SendWithOptions(host, port, user, pass, from, to, subject, body, Options{})
}

// SendWithOptions is Send + functional options. Currently only ProxyURL is
// supported; mismatched scheme falls back to direct dial with a warning.
func SendWithOptions(host string, port int, user, pass *string, from, to, subject, body string, opts Options) {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))

	// ヘッダーフィールドのCRLFインジェクション対策
	from = sanitizeHeaderValue(from)
	to = sanitizeHeaderValue(to)
	subject = sanitizeHeaderValue(subject)

	var auth gosmtp.Auth
	if user != nil && pass != nil && *user != "" {
		auth = gosmtp.PlainAuth("", *user, *pass, host)
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		from, to, subject, body)

	tlsConfig := &tls.Config{ServerName: host}
	conn, err := dialSMTP(addr, opts.ProxyURL, 10*time.Second)
	if err != nil {
		slog.Warn("email: dial failed", "error", err, "proxyConfigured", opts.ProxyURL != "")
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

// dialSMTP opens a TCP connection to addr (SMTP server). When proxyURL is
// set, the connection is tunnelled via the named proxy:
//
//   - socks5://[user:pass@]host:port → SOCKS5 (golang.org/x/net/proxy)
//   - socks5h:// は socks5 と同じ扱い (Go 標準では区別しない)
//   - http://host:port / https://host:port → HTTP CONNECT tunnel
//
// 不明な scheme / parse 失敗時は warn ログを出して direct dial にフォール
// バックする (proxy 設定ミスでメール送信全停止になるよりは、direct で
// 送れる経路があれば送る方を優先する)。
func dialSMTP(addr, proxyURL string, timeout time.Duration) (net.Conn, error) {
	if proxyURL == "" {
		return net.DialTimeout("tcp", addr, timeout)
	}
	u, err := url.Parse(proxyURL)
	if err != nil || u.Host == "" {
		slog.Warn("email: invalid proxySmtp URL, falling back to direct dial",
			"proxySmtp", proxyURL, "err", err)
		return net.DialTimeout("tcp", addr, timeout)
	}
	switch strings.ToLower(u.Scheme) {
	case "socks5", "socks5h":
		return dialSOCKS5(u, addr, timeout)
	case "http", "https":
		return dialHTTPConnect(u, addr, timeout)
	default:
		slog.Warn("email: unsupported proxySmtp scheme, falling back to direct dial",
			"scheme", u.Scheme)
		return net.DialTimeout("tcp", addr, timeout)
	}
}

// dialSOCKS5 wraps golang.org/x/net/proxy.SOCKS5 with the configured
// timeout. user/pass は URL の userinfo から取り出す (空なら認証無し)。
func dialSOCKS5(u *url.URL, addr string, timeout time.Duration) (net.Conn, error) {
	var auth *proxy.Auth
	if u.User != nil {
		pw, _ := u.User.Password()
		auth = &proxy.Auth{User: u.User.Username(), Password: pw}
	}
	d, err := proxy.SOCKS5("tcp", u.Host, auth, &net.Dialer{Timeout: timeout})
	if err != nil {
		return nil, fmt.Errorf("socks5 setup: %w", err)
	}
	// proxy.Dialer.Dial は context をサポートしないが、x/net/proxy の
	// SOCKS5 内部 Dialer に Timeout を渡しているので接続全体に伝播する。
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if cd, ok := d.(proxy.ContextDialer); ok {
		return cd.DialContext(ctx, "tcp", addr)
	}
	return d.Dial("tcp", addr)
}

// dialHTTPConnect opens a TCP connection to the HTTP proxy and issues a
// CONNECT request to tunnel through to addr. TLS over HTTP CONNECT (https://
// proxy) は proxy への接続を TLS で包んでから CONNECT を送る; SMTP server
// 側の StartTLS とは独立。
func dialHTTPConnect(u *url.URL, addr string, timeout time.Duration) (net.Conn, error) {
	proxyHost := u.Host
	if _, _, splitErr := net.SplitHostPort(proxyHost); splitErr != nil {
		port := "80"
		if strings.EqualFold(u.Scheme, "https") {
			port = "443"
		}
		proxyHost = net.JoinHostPort(u.Hostname(), port)
	}

	dialer := &net.Dialer{Timeout: timeout}
	var conn net.Conn
	var err error
	if strings.EqualFold(u.Scheme, "https") {
		conn, err = tls.DialWithDialer(dialer, "tcp", proxyHost, &tls.Config{ServerName: u.Hostname()})
	} else {
		conn, err = dialer.Dial("tcp", proxyHost)
	}
	if err != nil {
		return nil, fmt.Errorf("dial proxy %s: %w", proxyHost, err)
	}

	req := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n", addr, addr)
	if u.User != nil {
		// Basic auth — RFC 7617 base64(user:pass). 空 user / 空 pass は省略。
		username := u.User.Username()
		password, _ := u.User.Password()
		if username != "" || password != "" {
			creds := username + ":" + password
			encoded := encodeBasicAuth(creds)
			req += "Proxy-Authorization: Basic " + encoded + "\r\n"
		}
	}
	req += "\r\n"

	if _, err := conn.Write([]byte(req)); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("write CONNECT: %w", err)
	}

	// CONNECT 応答 1 行目のみで判定。"HTTP/1.1 200" を含めば OK、それ以外は
	// fail。残りの header 行は捨て (空行 or EOF まで読み進めても良いが、
	// SMTP 用途では proxy 応答後すぐに smtp プロトコルが流れ始めるので
	// 単純実装で十分)。
	if err := readCONNECTResponse(conn); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

// readCONNECTResponse parses the proxy's HTTP/1.1 CONNECT response. Returns
// nil on 2xx (tunnel established), error otherwise.
func readCONNECTResponse(conn net.Conn) error {
	buf := make([]byte, 4096)
	if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return fmt.Errorf("set read deadline: %w", err)
	}
	defer func() { _ = conn.SetReadDeadline(time.Time{}) }()

	n, err := conn.Read(buf)
	if err != nil && n == 0 {
		return fmt.Errorf("read CONNECT response: %w", err)
	}
	resp := string(buf[:n])
	// 1 行目: "HTTP/1.1 200 Connection established"
	idx := strings.Index(resp, "\r\n")
	if idx < 0 {
		return fmt.Errorf("malformed CONNECT response: %q", resp)
	}
	statusLine := resp[:idx]
	parts := strings.SplitN(statusLine, " ", 3)
	if len(parts) < 2 || !strings.HasPrefix(parts[1], "2") {
		return fmt.Errorf("CONNECT failed: %s", statusLine)
	}
	return nil
}

// encodeBasicAuth returns the base64 form of "user:pass" for the
// Proxy-Authorization header.
func encodeBasicAuth(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// errProxyTunnelFailed is exported for tests. (Currently dial errors are
// wrapped via fmt.Errorf, but a sentinel kept as an extension point.)
var errProxyTunnelFailed = errors.New("proxy tunnel failed")

var _ = errProxyTunnelFailed
