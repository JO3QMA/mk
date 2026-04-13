package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseTrustProxy_ValidCIDRs(t *testing.T) {
	opts := parseTrustProxy([]string{"10.0.0.0/8", "192.168.0.0/16"})
	assert.Len(t, opts, 2)
}

func TestParseTrustProxy_InvalidCIDR(t *testing.T) {
	// 無効なCIDRはスキップされる
	opts := parseTrustProxy([]string{"10.0.0.0/8", "invalid", "192.168.0.0/16"})
	assert.Len(t, opts, 2)
}

func TestParseTrustProxy_Empty(t *testing.T) {
	opts := parseTrustProxy(nil)
	assert.Empty(t, opts)
}

func TestParseTrustProxy_IPv6(t *testing.T) {
	opts := parseTrustProxy([]string{"::1/128", "fc00::/7"})
	assert.Len(t, opts, 2)
}

func TestParseTrustProxy_AllInvalid(t *testing.T) {
	opts := parseTrustProxy([]string{"not-a-cidr", "also-bad"})
	assert.Empty(t, opts)
}
