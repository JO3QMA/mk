package smtp

import (
	"bufio"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSanitizeHeaderValue(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain value is unchanged",
			in:   "user@example.com",
			want: "user@example.com",
		},
		{
			name: "strips LF injection",
			in:   "victim@example.com\nBcc: attacker@evil.com",
			want: "victim@example.comBcc: attacker@evil.com",
		},
		{
			name: "strips CR injection",
			in:   "subject\rSubject: spoofed",
			want: "subjectSubject: spoofed",
		},
		{
			name: "strips CRLF injection",
			in:   "victim@example.com\r\nBcc: attacker@evil.com",
			want: "victim@example.comBcc: attacker@evil.com",
		},
		{
			name: "empty input returns empty",
			in:   "",
			want: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeHeaderValue(tc.in)
			if got != tc.want {
				t.Errorf("sanitizeHeaderValue(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// fakeSMTP is a minimal SMTP server for exercising the full Send dialog
// without depending on a real mail server.
type fakeSMTP struct {
	ln        net.Listener
	offerAuth bool
	// rejectAt: "MAIL", "RCPT", "DATA", or "" for happy path.
	rejectAt string
	mu       sync.Mutex
	received []string
}

func newFakeSMTP(t *testing.T, offerAuth bool) *fakeSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	f := &fakeSMTP{ln: ln, offerAuth: offerAuth}
	t.Cleanup(func() { _ = ln.Close() })
	go f.serve()
	return f
}

func newRejectingSMTP(t *testing.T, rejectAt string) *fakeSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	f := &fakeSMTP{ln: ln, rejectAt: rejectAt}
	t.Cleanup(func() { _ = ln.Close() })
	go f.serve()
	return f
}

func (f *fakeSMTP) port() int {
	_, portStr, _ := net.SplitHostPort(f.ln.Addr().String())
	p, _ := strconv.Atoi(portStr)
	return p
}

func (f *fakeSMTP) record(s string) {
	f.mu.Lock()
	f.received = append(f.received, s)
	f.mu.Unlock()
}

func (f *fakeSMTP) messages() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]string, len(f.received))
	copy(cp, f.received)
	return cp
}

func (f *fakeSMTP) serve() {
	conn, err := f.ln.Accept()
	if err != nil {
		return
	}
	defer conn.Close()
	r := bufio.NewReader(conn)
	fmt.Fprint(conn, "220 localhost ESMTP\r\n")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		trimmed := strings.TrimRight(line, "\r\n")
		upper := strings.ToUpper(trimmed)
		switch {
		case strings.HasPrefix(upper, "EHLO"):
			if f.offerAuth {
				fmt.Fprint(conn, "250-localhost\r\n250 AUTH PLAIN\r\n")
			} else {
				fmt.Fprint(conn, "250 localhost\r\n")
			}
		case strings.HasPrefix(upper, "STARTTLS"):
			fmt.Fprint(conn, "502 STARTTLS not supported\r\n")
		case strings.HasPrefix(upper, "AUTH"):
			f.record("AUTH")
			if f.rejectAt == "AUTH" {
				fmt.Fprint(conn, "535 authentication failed\r\n")
				return
			}
			fmt.Fprint(conn, "235 2.7.0 Authentication successful\r\n")
		case strings.HasPrefix(upper, "MAIL FROM"):
			f.record(trimmed)
			if f.rejectAt == "MAIL" {
				fmt.Fprint(conn, "550 mailbox unavailable\r\n")
				return
			}
			fmt.Fprint(conn, "250 OK\r\n")
		case strings.HasPrefix(upper, "RCPT TO"):
			f.record(trimmed)
			if f.rejectAt == "RCPT" {
				fmt.Fprint(conn, "550 mailbox unavailable\r\n")
				return
			}
			fmt.Fprint(conn, "250 OK\r\n")
		case strings.HasPrefix(upper, "DATA"):
			if f.rejectAt == "DATA" {
				fmt.Fprint(conn, "554 transaction failed\r\n")
				return
			}
			fmt.Fprint(conn, "354 End data with <CR><LF>.<CR><LF>\r\n")
			var body strings.Builder
			for {
				dline, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if dline == ".\r\n" {
					break
				}
				body.WriteString(dline)
			}
			f.record(body.String())
			fmt.Fprint(conn, "250 OK\r\n")
		case strings.HasPrefix(upper, "QUIT"):
			fmt.Fprint(conn, "221 Bye\r\n")
			return
		default:
			fmt.Fprint(conn, "250 OK\r\n")
		}
	}
}

func waitForMessages(f *fakeSMTP, min int) []string {
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if len(f.messages()) >= min {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	return f.messages()
}

func TestSend_DeliversAndSanitizesHeaders(t *testing.T) {
	srv := newFakeSMTP(t, false)

	// ヘッダーに改行を仕込んでも注入されないことを検証する。
	Send("127.0.0.1", srv.port(), nil, nil,
		"from@example.com\r\nBcc: evil@example.com",
		"to@example.com",
		"Hello\nInjected: header",
		"body text")

	msgs := waitForMessages(srv, 3)
	if len(msgs) < 3 {
		t.Fatalf("expected at least 3 SMTP exchanges, got %d: %v", len(msgs), msgs)
	}

	// 改行が除去されたことでMAIL FROMが単一コマンドになり、追加のMAIL FROMが
	// 発行されていないことを確認する (注入成功なら2回発行される)。
	var mailFromCount int
	for _, m := range msgs {
		if strings.HasPrefix(m, "MAIL FROM:") {
			mailFromCount++
		}
	}
	if mailFromCount != 1 {
		t.Errorf("expected exactly 1 MAIL FROM (injection neutralized), got %d: %v", mailFromCount, msgs)
	}
	// DATA本文では連結済みのサニタイズ結果が入っていること。
	body := msgs[2]
	if !strings.Contains(body, "From: from@example.comBcc: evil@example.com") {
		t.Errorf("body missing sanitized From header: %q", body)
	}
	if !strings.Contains(body, "Subject: HelloInjected: header") {
		t.Errorf("body missing sanitized Subject: %q", body)
	}
	if !strings.Contains(body, "body text") {
		t.Errorf("body text missing: %q", body)
	}
}

func TestSend_WithAuth(t *testing.T) {
	srv := newFakeSMTP(t, true)
	user, pass := "u", "p"

	Send("127.0.0.1", srv.port(), &user, &pass,
		"from@example.com", "to@example.com", "subj", "body")

	msgs := waitForMessages(srv, 4)
	if !slices.Contains(msgs, "AUTH") {
		t.Errorf("expected AUTH to be issued, got %v", msgs)
	}
}

func TestSend_RejectionPathsAreBestEffort(t *testing.T) {
	// サーバ側が途中でエラー応答してもpanicせずSendが終了することを確認する。
	for _, rej := range []string{"MAIL", "RCPT", "DATA"} {
		t.Run(rej, func(t *testing.T) {
			srv := newRejectingSMTP(t, rej)
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Send panicked on reject=%s: %v", rej, r)
				}
			}()
			Send("127.0.0.1", srv.port(), nil, nil,
				"from@example.com", "to@example.com", "subj", "body")
		})
	}
}

func TestSend_AuthFailureIsBestEffort(t *testing.T) {
	// サーバがAUTHを拒否してもpanicせず終了することを確認する。
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	f := &fakeSMTP{ln: ln, offerAuth: true, rejectAt: "AUTH"}
	go f.serve()

	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portStr)
	user, pass := "u", "p"

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Send panicked: %v", r)
		}
	}()
	Send("127.0.0.1", port, &user, &pass,
		"from@example.com", "to@example.com", "subj", "body")
}

func TestSend_DialFailureIsBestEffort(t *testing.T) {
	// ポート未割り当てでdial失敗時にpanicせずreturnすることを確認する。
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Send panicked: %v", r)
		}
	}()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portStr)
	_ = ln.Close()
	Send("127.0.0.1", port, nil, nil, "from@example.com", "to@example.com", "subj", "body")
}

// #496: HTTP CONNECT proxy 経由で SMTP サーバに接続できることを確認する。
// proxy は CONNECT を受けた後そのまま target SMTP との bidirectional pipe
// になる minimum 実装。target SMTP は fakeSMTP を流用。
func TestSendWithOptions_HTTPConnectProxy(t *testing.T) {
	srv := newFakeSMTP(t, false)
	targetAddr := fmt.Sprintf("127.0.0.1:%d", srv.port())

	proxyLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen proxy: %v", err)
	}
	t.Cleanup(func() { _ = proxyLn.Close() })

	go func() {
		client, err := proxyLn.Accept()
		if err != nil {
			return
		}
		defer client.Close()
		// CONNECT request を読み捨てて 200 を返し、target に pipe する。
		r := bufio.NewReader(client)
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			if line == "\r\n" {
				break
			}
		}
		fmt.Fprint(client, "HTTP/1.1 200 Connection established\r\n\r\n")
		target, err := net.Dial("tcp", targetAddr)
		if err != nil {
			return
		}
		defer target.Close()
		// pipe both directions
		done := make(chan struct{}, 2)
		go func() {
			_, _ = copyConn(target, r)
			done <- struct{}{}
		}()
		go func() {
			_, _ = copyConn(client, target)
			done <- struct{}{}
		}()
		<-done
	}()

	proxyURL := fmt.Sprintf("http://%s", proxyLn.Addr().String())
	SendWithOptions("127.0.0.1", srv.port(), nil, nil,
		"from@example.com", "to@example.com", "subj", "body",
		Options{ProxyURL: proxyURL})

	msgs := waitForMessages(srv, 3)
	if len(msgs) < 3 {
		t.Fatalf("expected ≥3 SMTP exchanges via proxy, got %d: %v", len(msgs), msgs)
	}
}

// 不正な proxy URL が指定された場合は warn ログを出して direct dial に
// fallback する。SMTP 配送そのものは止まらない (best-effort 方針)。
func TestSendWithOptions_InvalidProxyFallsBackToDirect(t *testing.T) {
	srv := newFakeSMTP(t, false)
	SendWithOptions("127.0.0.1", srv.port(), nil, nil,
		"from@example.com", "to@example.com", "subj", "body",
		Options{ProxyURL: "://not-a-url"})
	if len(waitForMessages(srv, 3)) < 3 {
		t.Fatalf("expected SMTP delivery via direct fallback")
	}
}

// 未対応 scheme (例: ftp) も direct fallback する。
func TestSendWithOptions_UnsupportedSchemeFallsBackToDirect(t *testing.T) {
	srv := newFakeSMTP(t, false)
	SendWithOptions("127.0.0.1", srv.port(), nil, nil,
		"from@example.com", "to@example.com", "subj", "body",
		Options{ProxyURL: "ftp://127.0.0.1:21"})
	if len(waitForMessages(srv, 3)) < 3 {
		t.Fatalf("expected SMTP delivery via direct fallback")
	}
}

// HTTP CONNECT が non-2xx で返してきた場合は接続失敗扱いで panic せず
// 終了する。
func TestSendWithOptions_HTTPConnectRejected(t *testing.T) {
	proxyLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen proxy: %v", err)
	}
	t.Cleanup(func() { _ = proxyLn.Close() })

	go func() {
		client, err := proxyLn.Accept()
		if err != nil {
			return
		}
		defer client.Close()
		r := bufio.NewReader(client)
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			if line == "\r\n" {
				break
			}
		}
		fmt.Fprint(client, "HTTP/1.1 403 Forbidden\r\n\r\n")
	}()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Send panicked on proxy 403: %v", r)
		}
	}()
	SendWithOptions("127.0.0.1", 25, nil, nil,
		"from@example.com", "to@example.com", "subj", "body",
		Options{ProxyURL: "http://" + proxyLn.Addr().String()})
}

func TestEncodeBasicAuth(t *testing.T) {
	if got := encodeBasicAuth("alice:secret"); got != "YWxpY2U6c2VjcmV0" {
		t.Errorf("encodeBasicAuth: got %q", got)
	}
}

// HTTP CONNECT proxy で Proxy-Authorization Basic ヘッダが付与されることを
// 確認する。proxy 側で受信した CONNECT request 全体を録画してアサート。
func TestSendWithOptions_HTTPConnectBasicAuth(t *testing.T) {
	srv := newFakeSMTP(t, false)
	targetAddr := fmt.Sprintf("127.0.0.1:%d", srv.port())

	proxyLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen proxy: %v", err)
	}
	t.Cleanup(func() { _ = proxyLn.Close() })

	var capturedReq strings.Builder
	go func() {
		client, err := proxyLn.Accept()
		if err != nil {
			return
		}
		defer client.Close()
		r := bufio.NewReader(client)
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			capturedReq.WriteString(line)
			if line == "\r\n" {
				break
			}
		}
		fmt.Fprint(client, "HTTP/1.1 200 Connection established\r\n\r\n")
		target, err := net.Dial("tcp", targetAddr)
		if err != nil {
			return
		}
		defer target.Close()
		done := make(chan struct{}, 2)
		go func() { _, _ = copyConn(target, r); done <- struct{}{} }()
		go func() { _, _ = copyConn(client, target); done <- struct{}{} }()
		<-done
	}()

	proxyURL := fmt.Sprintf("http://alice:secret@%s", proxyLn.Addr().String())
	SendWithOptions("127.0.0.1", srv.port(), nil, nil,
		"from@example.com", "to@example.com", "subj", "body",
		Options{ProxyURL: proxyURL})
	_ = waitForMessages(srv, 3)

	got := capturedReq.String()
	if !strings.Contains(got, "Proxy-Authorization: Basic YWxpY2U6c2VjcmV0") {
		t.Errorf("Proxy-Authorization header missing or wrong: %q", got)
	}
}

// SOCKS5 with username/password auth: x/net/proxy が Auth method 0x02 を
// negotiate するので fake server も対応する。
func TestSendWithOptions_SOCKS5WithAuth(t *testing.T) {
	srv := newFakeSMTP(t, false)
	targetPort := srv.port()

	proxyLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen socks5: %v", err)
	}
	t.Cleanup(func() { _ = proxyLn.Close() })

	go func() {
		client, err := proxyLn.Accept()
		if err != nil {
			return
		}
		defer client.Close()
		// Greeting: read VER + NMETHODS
		hdr := make([]byte, 2)
		if _, err := client.Read(hdr); err != nil {
			return
		}
		methods := make([]byte, int(hdr[1]))
		_, _ = client.Read(methods)
		// Select user/pass auth (0x02)
		_, _ = client.Write([]byte{0x05, 0x02})

		// Read sub-negotiation: VER=1, ULEN, UNAME, PLEN, PASSWD
		auth := make([]byte, 256)
		n, _ := client.Read(auth)
		_ = n
		// 認証成功: VER=1, STATUS=0
		_, _ = client.Write([]byte{0x01, 0x00})

		// CONNECT (VER+CMD+RSV+ATYP+ADDR+PORT = 10 bytes for IPv4)
		req := make([]byte, 10)
		_, _ = client.Read(req)
		_, _ = client.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})

		target, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", targetPort))
		if err != nil {
			return
		}
		defer target.Close()
		done := make(chan struct{}, 2)
		go func() { _, _ = copyConn(target, client); done <- struct{}{} }()
		go func() { _, _ = copyConn(client, target); done <- struct{}{} }()
		<-done
	}()

	proxyURL := fmt.Sprintf("socks5://alice:secret@%s", proxyLn.Addr().String())
	SendWithOptions("127.0.0.1", targetPort, nil, nil,
		"from@example.com", "to@example.com", "subj", "body",
		Options{ProxyURL: proxyURL})
	if len(waitForMessages(srv, 3)) < 3 {
		t.Fatalf("expected SMTP delivery via authenticated SOCKS5")
	}
}

// CONNECT 応答が malformed (CRLF 無し) のとき接続は close される。
func TestSendWithOptions_HTTPConnectMalformedResponse(t *testing.T) {
	proxyLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen proxy: %v", err)
	}
	t.Cleanup(func() { _ = proxyLn.Close() })
	go func() {
		client, err := proxyLn.Accept()
		if err != nil {
			return
		}
		defer client.Close()
		// Read CONNECT (until empty line) then return malformed (no CRLF)
		r := bufio.NewReader(client)
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			if line == "\r\n" {
				break
			}
		}
		fmt.Fprint(client, "BLOB-NOT-HTTP")
	}()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Send panicked: %v", r)
		}
	}()
	SendWithOptions("127.0.0.1", 25, nil, nil,
		"from@example.com", "to@example.com", "subj", "body",
		Options{ProxyURL: "http://" + proxyLn.Addr().String()})
}

// proxy URL に port 省略 → scheme 既定 (http=80) を補う経路。port 80 への
// 接続は dial timeout か connection refused になるが、SendWithOptions は
// best-effort で panic せず終了する。
func TestSendWithOptions_HTTPProxyDefaultPort(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Send panicked on default port path: %v", r)
		}
	}()
	SendWithOptions("127.0.0.1", 25, nil, nil,
		"from@example.com", "to@example.com", "subj", "body",
		Options{ProxyURL: "http://127.0.0.1"}) // no port → default 80
}

// HTTPS proxy URL は tls.DialWithDialer 経路に入る。proxy 側に TLS 立てる
// テストは大変なので、TLS handshake 失敗 (= proxy が plain TCP) で
// 想定通り dial error になることを確認する (= HTTPS branch コード実行
// 確認のみ、実 SMTP は届かない)。
func TestSendWithOptions_HTTPSProxyTLSHandshakeFails(t *testing.T) {
	proxyLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen proxy: %v", err)
	}
	t.Cleanup(func() { _ = proxyLn.Close() })
	go func() {
		c, _ := proxyLn.Accept()
		if c != nil {
			c.Close()
		}
	}()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Send panicked: %v", r)
		}
	}()
	SendWithOptions("127.0.0.1", 25, nil, nil,
		"from@example.com", "to@example.com", "subj", "body",
		Options{ProxyURL: "https://" + proxyLn.Addr().String()})
}

// SOCKS5 proxy 経由の dial 経路を確認。fake SOCKS5 server が CONNECT を
// 受けて target SMTP に proxy する。
func TestSendWithOptions_SOCKS5Proxy(t *testing.T) {
	srv := newFakeSMTP(t, false)
	targetPort := srv.port()

	proxyLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen socks5: %v", err)
	}
	t.Cleanup(func() { _ = proxyLn.Close() })

	go func() {
		client, err := proxyLn.Accept()
		if err != nil {
			return
		}
		defer client.Close()
		// SOCKS5 hand-shake (no-auth)
		hdr := make([]byte, 2)
		if _, err := client.Read(hdr); err != nil {
			return
		}
		nMethods := int(hdr[1])
		methods := make([]byte, nMethods)
		_, _ = client.Read(methods)
		// 0x05 0x00 = no-auth selected
		_, _ = client.Write([]byte{0x05, 0x00})

		// CONNECT request: VER=5, CMD=1 (CONNECT), RSV=0, ATYP=1 (IPv4),
		// DST.ADDR=4 bytes, DST.PORT=2 bytes
		req := make([]byte, 10)
		if _, err := client.Read(req); err != nil {
			return
		}
		// Reply: VER=5, REP=0 (success), RSV=0, ATYP=1, BND.ADDR=0.0.0.0, BND.PORT=0
		_, _ = client.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})

		// Pipe to target SMTP server.
		target, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", targetPort))
		if err != nil {
			return
		}
		defer target.Close()
		done := make(chan struct{}, 2)
		go func() { _, _ = copyConn(target, client); done <- struct{}{} }()
		go func() { _, _ = copyConn(client, target); done <- struct{}{} }()
		<-done
	}()

	proxyURL := fmt.Sprintf("socks5://%s", proxyLn.Addr().String())
	SendWithOptions("127.0.0.1", targetPort, nil, nil,
		"from@example.com", "to@example.com", "subj", "body",
		Options{ProxyURL: proxyURL})
	if len(waitForMessages(srv, 3)) < 3 {
		t.Fatalf("expected SMTP delivery via SOCKS5 proxy")
	}
}

// copyConn shuttles bytes from src to dst. Local helper for the proxy test
// (avoids pulling in io.Copy's interface dance).
func copyConn(dst net.Conn, src interface{ Read([]byte) (int, error) }) (int64, error) {
	buf := make([]byte, 4096)
	var total int64
	for {
		n, err := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return total, werr
			}
			total += int64(n)
		}
		if err != nil {
			return total, err
		}
	}
}
