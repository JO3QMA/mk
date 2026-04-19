// Package safehttp provides shared HTTP helpers used across outbound
// fetchers (ActivityPub, WebFinger, URL preview, media proxy). Centralises
// SSRF protection and response size caps so hardening improvements propagate
// everywhere in one place.
package safehttp

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// privateRanges lists IPv4/IPv6 CIDR blocks considered private or reserved.
var privateRanges []*net.IPNet

func init() {
	cidrs := []string{
		// IPv4 private / reserved
		"0.0.0.0/8",
		"10.0.0.0/8",
		"100.64.0.0/10",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"172.16.0.0/12",
		"192.0.0.0/24",
		"192.0.2.0/24",
		"192.168.0.0/16",
		"198.18.0.0/15",
		"198.51.100.0/24",
		"203.0.113.0/24",
		"224.0.0.0/4",
		"240.0.0.0/4",
		// IPv6 private / reserved
		"::1/128",
		"fe80::/10",
		"fc00::/7",
	}
	for _, cidr := range cidrs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			panic("invalid CIDR in privateRanges: " + cidr)
		}
		privateRanges = append(privateRanges, ipNet)
	}
}

// ErrSSRFBlocked is returned when a connection to a private/reserved IP is blocked.
var ErrSSRFBlocked = fmt.Errorf("safehttp: connection to private IP blocked")

// NewSSRFSafeTransport returns an *http.Transport with a custom DialContext
// that resolves DNS first and rejects connections to private/reserved IPs.
// allowedCIDRs は config.AllowedPrivateNetworks に対応し、明示的に許可する CIDR リスト。
func NewSSRFSafeTransport(allowedCIDRs []string) *http.Transport {
	var allowedNets []*net.IPNet
	for _, cidr := range allowedCIDRs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		allowedNets = append(allowedNets, ipNet)
	}

	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("safehttp: invalid address %q: %w", addr, err)
			}

			// DNS解決して実IPを取得
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("safehttp: DNS lookup failed for %q: %w", host, err)
			}

			// 全解決IPがプライベートでないか検証
			for _, ipAddr := range ips {
				if isPrivateIP(ipAddr.IP, allowedNets) {
					return nil, ErrSSRFBlocked
				}
			}

			// 検証済みIPで接続（最初に成功したものを使う）
			for _, ipAddr := range ips {
				target := net.JoinHostPort(ipAddr.IP.String(), port)
				conn, dialErr := dialer.DialContext(ctx, network, target)
				if dialErr == nil {
					return conn, nil
				}
				err = dialErr
			}
			return nil, err
		},
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		// http.DefaultTransport と同様に HTTP/2 negotiation を有効にする。
		// 既定の *http.Transport{} ではデフォルト false のため、AP fetch が
		// HTTP/1.1 のみに退化するのを避ける (#323 Devin review)。
		ForceAttemptHTTP2: true,
	}
}

// isPrivateIP returns true if ip falls within a private/reserved range
// and is NOT covered by any of the allowedNets exceptions.
func isPrivateIP(ip net.IP, allowedNets []*net.IPNet) bool {
	// IPv4-mapped IPv6 (::ffff:x.x.x.x) の中身を取り出してIPv4として判定
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}

	// allowedNets に含まれるIPは許可
	for _, allowed := range allowedNets {
		if allowed.Contains(ip) {
			return false
		}
	}

	// プライベート/予約済みレンジに含まれるか判定
	for _, r := range privateRanges {
		if r.Contains(ip) {
			return true
		}
	}
	return false
}
