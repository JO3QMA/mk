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
	"net/url"
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

// transportOptions accumulates state from functional Option values.
type transportOptions struct {
	proxyURL    string
	bypassHosts []string
}

// Option configures NewSSRFSafeTransport. Use WithProxy to enable forward
// proxy support; pass no options for the default SSRF-safe direct transport.
type Option func(*transportOptions)

// WithProxy wires a forward HTTP proxy and an optional bypass list. proxyURL
// must be a full URL parseable by url.Parse (e.g. http://127.0.0.1:3128). When
// proxyURL is empty the option is a no-op so callers can pass config values
// straight through. bypassHosts is matched as exact case-sensitive hostname
// equality, mirroring upstream Misskey's `proxyBypassHosts.includes(...)`.
func WithProxy(proxyURL string, bypassHosts []string) Option {
	return func(o *transportOptions) {
		o.proxyURL = proxyURL
		o.bypassHosts = bypassHosts
	}
}

// NewSSRFSafeTransport returns an *http.Transport with a custom DialContext
// that resolves DNS first and rejects connections to private/reserved IPs.
// allowedCIDRs は config.AllowedPrivateNetworks に対応し、明示的に許可する CIDR リスト。
// Functional opts (現状 WithProxy のみ) で外向き forward proxy 経由を有効化できる。
func NewSSRFSafeTransport(allowedCIDRs []string, opts ...Option) *http.Transport {
	o := transportOptions{}
	for _, fn := range opts {
		fn(&o)
	}

	var allowedNets []*net.IPNet
	for _, cidr := range allowedCIDRs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		allowedNets = append(allowedNets, ipNet)
	}

	// proxy 設定があれば事前に URL を一度だけ parse する。失敗したものは
	// 無視 (proxy 不使用) して direct fallback。起動時に config 経由で
	// 渡るので毎リクエスト parse する必要は無い。
	var proxyURL *url.URL
	var proxyAddr string // host:port 形式 (DialContext での比較用)
	if o.proxyURL != "" {
		if u, err := url.Parse(o.proxyURL); err == nil && u.Host != "" {
			proxyURL = u
			proxyAddr = u.Host
			// URL に明示ポートが無い場合は scheme 既定を補う。後段の
			// addr 比較で `host:80` のような正規化形と一致させるため。
			// IPv6 host は u.Host が "[::1]" のように bracket 付きで
			// 返るため net.JoinHostPort には bracket を剥がした
			// u.Hostname() を渡す (二重 bracket を避ける)。
			if _, _, splitErr := net.SplitHostPort(proxyAddr); splitErr != nil {
				if u.Scheme == "https" {
					proxyAddr = net.JoinHostPort(u.Hostname(), "443")
				} else {
					proxyAddr = net.JoinHostPort(u.Hostname(), "80")
				}
			}
		}
	}
	bypass := make(map[string]struct{}, len(o.bypassHosts))
	for _, h := range o.bypassHosts {
		if h != "" {
			bypass[h] = struct{}{}
		}
	}

	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	tr := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// proxy 経由のリクエストでは Transport.Proxy callback の結果に
			// 沿って http.Transport が proxy host:port で DialContext を
			// 呼ぶ。proxy はオペレーターが明示設定したエンドポイントなので
			// SSRF check (private IP 拒否) は適用しない。bypass 経路や
			// proxy 未指定の direct dial には従来通り SSRF を適用する。
			if proxyAddr != "" && addr == proxyAddr {
				return dialer.DialContext(ctx, network, addr)
			}

			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("safehttp: invalid address %q: %w", addr, err)
			}

			// DNS解決して実IPを取得。Goのresolverは「nil err + 空slice」を
			// 返さない契約のため、以降のloopは必ず1回以上実行される。
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
	if proxyURL != nil {
		// upstream Misskey の HttpRequestService.getAgentByUrl と同じく
		// proxyBypassHosts に含まれる hostname (exact match) は proxy を
		// 経由せず direct で出す。それ以外は proxyURL に CONNECT/forward。
		tr.Proxy = func(req *http.Request) (*url.URL, error) {
			if _, ok := bypass[req.URL.Hostname()]; ok {
				return nil, nil
			}
			return proxyURL, nil
		}
	}
	return tr
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
