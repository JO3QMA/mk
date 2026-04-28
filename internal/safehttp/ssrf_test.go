package safehttp

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsPrivateIP_IPv4Private(t *testing.T) {
	tests := []struct {
		name string
		ip   string
	}{
		{"loopback", "127.0.0.1"},
		{"loopback high", "127.255.255.255"},
		{"10.x", "10.0.0.1"},
		{"10.x high", "10.255.255.255"},
		{"172.16.x", "172.16.0.1"},
		{"172.31.x", "172.31.255.255"},
		{"192.168.x", "192.168.1.1"},
		{"link-local", "169.254.1.1"},
		{"zero network", "0.0.0.1"},
		{"CGN", "100.64.0.1"},
		{"documentation 192.0.2.x", "192.0.2.1"},
		{"documentation 198.51.100.x", "198.51.100.1"},
		{"documentation 203.0.113.x", "203.0.113.1"},
		{"benchmark 198.18.x", "198.18.0.1"},
		{"multicast", "224.0.0.1"},
		{"reserved 240.x", "240.0.0.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, isPrivateIP(net.ParseIP(tt.ip), nil), "%s should be private", tt.ip)
		})
	}
}

func TestIsPrivateIP_IPv6Private(t *testing.T) {
	tests := []struct {
		name string
		ip   string
	}{
		{"loopback", "::1"},
		{"link-local", "fe80::1"},
		{"unique-local fd", "fd00::1"},
		{"unique-local fc", "fc00::1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, isPrivateIP(net.ParseIP(tt.ip), nil), "%s should be private", tt.ip)
		})
	}
}

func TestIsPrivateIP_IPv4MappedIPv6(t *testing.T) {
	// ::ffff:127.0.0.1 はIPv4-mapped IPv6であり、中身の127.0.0.1で判定すべき
	ip := net.ParseIP("::ffff:127.0.0.1")
	assert.True(t, isPrivateIP(ip, nil))

	ip = net.ParseIP("::ffff:10.0.0.1")
	assert.True(t, isPrivateIP(ip, nil))
}

func TestIsPrivateIP_PublicIP(t *testing.T) {
	tests := []struct {
		name string
		ip   string
	}{
		{"Google DNS", "8.8.8.8"},
		{"Cloudflare DNS", "1.1.1.1"},
		{"random public", "203.104.209.7"},
		{"IPv6 public", "2001:4860:4860::8888"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.False(t, isPrivateIP(net.ParseIP(tt.ip), nil), "%s should be public", tt.ip)
		})
	}
}

func TestIsPrivateIP_AllowedCIDR(t *testing.T) {
	_, allowedNet, _ := net.ParseCIDR("10.0.0.0/24")
	allowedNets := []*net.IPNet{allowedNet}

	// 10.0.0.1 はプライベートだが allowedNets に含まれるので許可
	assert.False(t, isPrivateIP(net.ParseIP("10.0.0.1"), allowedNets))

	// 10.1.0.1 は allowedNets に含まれないのでブロック
	assert.True(t, isPrivateIP(net.ParseIP("10.1.0.1"), allowedNets))
}

func TestIsPrivateIP_AllowedCIDR_IPv6(t *testing.T) {
	_, allowedNet, _ := net.ParseCIDR("fd00::/64")
	allowedNets := []*net.IPNet{allowedNet}

	assert.False(t, isPrivateIP(net.ParseIP("fd00::1"), allowedNets))
	assert.True(t, isPrivateIP(net.ParseIP("fd01::1"), allowedNets))
}

func TestNewSSRFSafeTransport_InvalidCIDR(t *testing.T) {
	// 不正なCIDRは無視される（panicしない）
	tr := NewSSRFSafeTransport([]string{"invalid-cidr", "10.0.0.0/8"})
	assert.NotNil(t, tr)
}

func TestNewSSRFSafeTransport_NilCIDRs(t *testing.T) {
	tr := NewSSRFSafeTransport(nil)
	assert.NotNil(t, tr)
}

func TestNewSSRFSafeTransport_BlocksLoopback(t *testing.T) {
	// httptestサーバーは127.0.0.1で起動するのでSSRF保護でブロックされるはず
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	tr := NewSSRFSafeTransport(nil)
	client := &http.Client{Transport: tr}

	_, err := client.Get(ts.URL + "/test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "private IP blocked")
}

func TestNewSSRFSafeTransport_AllowsLoopbackWithCIDR(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	tr := NewSSRFSafeTransport([]string{"127.0.0.0/8"})
	client := &http.Client{Transport: tr}

	resp, err := client.Get(ts.URL + "/test")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestNewSSRFSafeTransport_DNSResolutionFailure(t *testing.T) {
	tr := NewSSRFSafeTransport(nil)
	client := &http.Client{Transport: tr}

	_, err := client.Get("http://nonexistent.invalid/test")
	assert.Error(t, err)
}

// proxy 経路で外向きリクエストが proxy server に届くこと、proxy 自体が
// プライベート IP でも SSRF block されないこと (#485)。
func TestNewSSRFSafeTransport_ForwardsThroughProxy(t *testing.T) {
	hits := 0
	// proxy 役の httptest server。HTTP の forward proxy は absolute-form
	// URL (例: GET http://example.com/foo HTTP/1.1) を受け取るので
	// req.URL を見て応答するだけで proxy として振る舞える。
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		// proxy は absolute-form URL を受け取る
		assert.Equal(t, "example.com", r.URL.Host)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("via-proxy"))
	}))
	defer proxy.Close()

	// httptest は 127.0.0.1 で起動するので allowedCIDRs 無しでも proxy
	// 接続自体は SSRF skip される (proxyAddr 一致)。
	tr := NewSSRFSafeTransport(nil, WithProxy(proxy.URL, nil))
	client := &http.Client{Transport: tr}

	resp, err := client.Get("http://example.com/path")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 1, hits)
}

// bypass list に列挙された host は proxy を経由せず direct dial になる。
// direct dial は SSRF check 対象なので、loopback への bypass は CIDR 許可
// が無い限りブロックされる。
func TestNewSSRFSafeTransport_BypassRoutesDirectAndKeepsSSRF(t *testing.T) {
	proxyHits := 0
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		proxyHits++
		w.WriteHeader(http.StatusOK)
	}))
	defer proxy.Close()

	directHits := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		directHits++
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	targetURL, err := url.Parse(target.URL)
	require.NoError(t, err)
	// bypass で direct dial になるが、target も loopback なので SSRF を
	// 許可するため CIDR 127.0.0.0/8 を allowedCIDRs に入れる。
	tr := NewSSRFSafeTransport(
		[]string{"127.0.0.0/8"},
		WithProxy(proxy.URL, []string{targetURL.Hostname()}),
	)
	client := &http.Client{Transport: tr}

	resp, err := client.Get(target.URL + "/x")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 0, proxyHits, "bypass-listed host must not go through proxy")
	assert.Equal(t, 1, directHits)
}

// proxy URL が不正な場合は proxy 設定が無視され direct dial にフォール
// バックする (起動時に panic させない)。
func TestNewSSRFSafeTransport_InvalidProxyURLFallsBack(t *testing.T) {
	tr := NewSSRFSafeTransport(nil, WithProxy("://not-a-url", nil))
	assert.NotNil(t, tr)
	assert.Nil(t, tr.Proxy, "invalid proxy URL must leave Transport.Proxy unset")
}

// WithProxy("", ...) は no-op (proxy 不使用)。
func TestNewSSRFSafeTransport_EmptyProxyURLNoOp(t *testing.T) {
	tr := NewSSRFSafeTransport(nil, WithProxy("", []string{"example.com"}))
	assert.Nil(t, tr.Proxy, "empty proxy URL must not enable Transport.Proxy")
}

// IPv6 host の proxy URL に明示 port が無いとき、u.Host は "[::1]"
// のように bracket 付きで返る。これを net.JoinHostPort にそのまま渡すと
// "[[::1]]:80" の二重 bracket になり、後段の DialContext 比較で proxy
// 一致と判定されず SSRF check が走る → loopback (::1) として block
// されてしまう。u.Hostname() で剥がしてから net.JoinHostPort に渡す
// ことで http.Transport が dial する正規形 ("[::1]:80") と一致する。
//
// 直接 proxyAddr を assert できないので、実 dial を試して
// ErrSSRFBlocked が返らない (= proxy 一致判定が機能している) ことで
// 間接的に検証する。dial 自体はループバック上の閉じた port なので
// "connection refused" 系のエラーになるが、それは bug fix の対象外。
func TestNewSSRFSafeTransport_IPv6ProxyDefaultPort(t *testing.T) {
	tr := NewSSRFSafeTransport(nil, WithProxy("http://[::1]", nil))
	require.NotNil(t, tr)
	require.NotNil(t, tr.Proxy)

	client := &http.Client{Transport: tr, Timeout: 2 * time.Second}
	_, err := client.Get("http://example.com/")
	require.Error(t, err, "dial to ::1:80 will fail (no listener) but must not be SSRF-blocked")
	assert.NotErrorIs(t, err, ErrSSRFBlocked,
		"proxy connection must skip SSRF check even when proxy host is IPv6 loopback without explicit port")
}
