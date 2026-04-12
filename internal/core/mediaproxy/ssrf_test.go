package mediaproxy

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

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
