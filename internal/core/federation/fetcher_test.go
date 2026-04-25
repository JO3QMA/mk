package federation

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shiroha-a/mk/internal/activitypub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// genTestRSAPEM produces a small RSA private key in PKCS#1 PEM format. Key
// size is intentionally small (1024-bit) to keep tests fast — never use
// these helpers in production paths.
func genTestRSAPEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	require.NoError(t, err)
	der := x509.MarshalPKCS1PrivateKey(key)
	return string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}))
}

// stubSigner returns canned credentials. err != nil simulates the
// "instance.actor not yet provisioned" branch (= ErrNoSigner)。
type stubSigner struct {
	keyID  string
	keyPEM string
	err    error
}

func (s *stubSigner) SignerCredentials() (string, string, error) {
	return s.keyID, s.keyPEM, s.err
}

func TestAPFetcher_FetchObject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/activity+json")
		_, _ = w.Write([]byte(`{"id":"x"}`))
	}))
	defer srv.Close()

	c := activitypub.NewClient(nil, "test")
	f := NewAPFetcher(c)
	body, err := f.FetchObject(srv.URL + "/users/alice")
	require.NoError(t, err)
	assert.Contains(t, string(body), "x")
}

// signer 未配線時は従来通り unsigned GET だけで動作する (#419 互換性)。
func TestAPFetcher_FetchObject_NoSigner_UnsignedOnly(t *testing.T) {
	var sawSig bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Signature") != "" {
			sawSig = true
		}
		w.Header().Set("Content-Type", "application/activity+json")
		_, _ = w.Write([]byte(`{"id":"x"}`))
	}))
	defer srv.Close()

	f := NewAPFetcher(activitypub.NewClient(nil, "test"))
	_, err := f.FetchObject(srv.URL + "/users/alice")
	require.NoError(t, err)
	assert.False(t, sawSig, "unsigned mode must not set Signature header")
}

// signer 配線済 + peer が signed GET を受け入れる場合は signed リクエストが
// 通り、unsigned へのフォールバックは発生しない (#419)。
func TestAPFetcher_FetchObject_SignedDefault_PeerAcceptsSigned(t *testing.T) {
	var calls int
	var firstHadSig bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 && r.Header.Get("Signature") != "" {
			firstHadSig = true
		}
		w.Header().Set("Content-Type", "application/activity+json")
		_, _ = w.Write([]byte(`{"id":"signed"}`))
	}))
	defer srv.Close()

	f := NewAPFetcher(activitypub.NewClient(nil, "test"))
	f.SetSigner(&stubSigner{
		keyID:  "https://local.example/users/sys#main-key",
		keyPEM: genTestRSAPEM(t),
	})
	body, err := f.FetchObject(srv.URL + "/users/alice")
	require.NoError(t, err)
	assert.Equal(t, 1, calls, "should not fall back when signed succeeded")
	assert.True(t, firstHadSig, "signed GET must include Signature header")
	assert.Contains(t, string(body), "signed")
}

// IceShrimp.NET 等の authorized-fetch peer (signed のみ受理) でも、署名
// 鍵が wire されていれば 1 発で通る。これが #419 の core fix。
func TestAPFetcher_FetchObject_SignedDefault_AuthorizedFetchPeer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Signature") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"missing signature"}`))
			return
		}
		w.Header().Set("Content-Type", "application/activity+json")
		_, _ = w.Write([]byte(`{"id":"ok"}`))
	}))
	defer srv.Close()

	f := NewAPFetcher(activitypub.NewClient(nil, "test"))
	f.SetSigner(&stubSigner{
		keyID:  "https://local.example/users/sys#main-key",
		keyPEM: genTestRSAPEM(t),
	})
	body, err := f.FetchObject(srv.URL + "/users/alice")
	require.NoError(t, err)
	assert.Contains(t, string(body), "ok")
}

// signed GET が失敗した時は従来の unsigned GET にフォールバックする (signature
// 検証が緩い peer や signer key 不在時の互換性確保)。
func TestAPFetcher_FetchObject_FallsBackToUnsignedOnSignedError(t *testing.T) {
	var unsignedHits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Signature") != "" {
			// 例えば peer 側で signature verification 失敗 → 4xx
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"bad sig"}`))
			return
		}
		unsignedHits++
		w.Header().Set("Content-Type", "application/activity+json")
		_, _ = w.Write([]byte(`{"id":"unsigned-ok"}`))
	}))
	defer srv.Close()

	f := NewAPFetcher(activitypub.NewClient(nil, "test"))
	f.SetSigner(&stubSigner{
		keyID:  "https://local.example/users/sys#main-key",
		keyPEM: genTestRSAPEM(t),
	})
	body, err := f.FetchObject(srv.URL + "/users/alice")
	require.NoError(t, err)
	assert.Equal(t, 1, unsignedHits, "expect single unsigned fallback")
	assert.Contains(t, string(body), "unsigned-ok")
}

// SignerCredentials がエラー (instance.actor 未プロビジョン等) を返す場合は
// 即 unsigned へ落ちる。ErrNoSigner のセマンティクスを担保する。
func TestAPFetcher_FetchObject_SignerErrorSkipsToUnsigned(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/activity+json")
		_, _ = w.Write([]byte(`{"id":"y"}`))
	}))
	defer srv.Close()

	f := NewAPFetcher(activitypub.NewClient(nil, "test"))
	f.SetSigner(&stubSigner{err: ErrNoSigner})
	body, err := f.FetchObject(srv.URL + "/users/alice")
	require.NoError(t, err)
	assert.Contains(t, string(body), "y")
}

// FetchUnsigned / FetchJSON が non-2xx で StatusError を返すことを確認する。
// IceShrimp.NET の 401 を上位で識別するための土台 (#419)。
func TestActivityPubClient_FetchUnsigned_NonOkReturnsStatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	_, err := activitypub.NewClient(nil, "test").FetchUnsigned(srv.URL + "/")
	require.Error(t, err)
	var se *activitypub.StatusError
	require.True(t, errorsAs(err, &se))
	assert.Equal(t, http.StatusUnauthorized, se.StatusCode)
}

// errorsAs is a tiny errors.As shim usable in test code without pulling in
// the whole errors package import here. Keeps the test file imports tidy.
func errorsAs(err error, target any) bool {
	se, ok := target.(**activitypub.StatusError)
	if !ok {
		return false
	}
	for e := err; e != nil; {
		if v, ok := e.(*activitypub.StatusError); ok {
			*se = v
			return true
		}
		// 単純な Wrap 想定: Errorf("...: %w", inner) を unwrap
		type unwrapper interface{ Unwrap() error }
		if u, ok := e.(unwrapper); ok {
			e = u.Unwrap()
			continue
		}
		break
	}
	return false
}

// 互換性: 既存の fmt.Errorf("unexpected status: ...").Error() 文字列依存
// callsite が無いか軽くチェックする (regression 防止)。
func TestStatusError_PreservesLegacyErrorString(t *testing.T) {
	se := &activitypub.StatusError{StatusCode: 401, Status: "401 Unauthorized", URL: "x"}
	assert.True(t, strings.HasPrefix(se.Error(), "unexpected status:"))
}

func TestAPFetcher_FetchHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><link rel="icon" href="/a.png"></head></html>`))
	}))
	defer srv.Close()

	c := activitypub.NewClient(nil, "test")
	f := NewAPFetcher(c)
	body, err := f.FetchHTML(srv.URL)
	require.NoError(t, err)
	assert.Contains(t, string(body), "<link rel=\"icon\"")
}
