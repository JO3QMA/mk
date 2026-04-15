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
