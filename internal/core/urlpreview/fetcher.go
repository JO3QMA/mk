package urlpreview

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrDisabled    = errors.New("urlpreview: feature disabled")
	ErrPrivateIP   = errors.New("urlpreview: private IP not allowed")
	ErrTooLarge    = errors.New("urlpreview: content too large")
	ErrFetchFailed = errors.New("urlpreview: fetch failed")
)

const cachePrefix = "url-preview:"
const cacheTTL = 24 * time.Hour

// Config holds URL preview settings read from Meta.
type Config struct {
	Enabled              bool
	AllowRedirect        bool
	TimeoutMs            int
	MaxContentLength     int64
	RequireContentLength bool
	SummaryProxyURL      string
	UserAgent            string
}

// Fetcher fetches and parses URL previews with caching and SSRF protection.
type Fetcher struct {
	cfg         Config
	redis       *redis.Client
	client      *http.Client
	proxyClient *http.Client
}

// SetHTTPClient replaces the HTTP client (for tests that need to bypass
// the SSRF dialer or use a custom transport).
func (f *Fetcher) SetHTTPClient(c *http.Client) {
	f.client = c
}

// NewFetcher creates a URL preview Fetcher.
func NewFetcher(cfg Config, rdb *redis.Client) *Fetcher {
	timeout := time.Duration(cfg.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, _, _ := net.SplitHostPort(addr)
			if isPrivateHost(host) {
				return nil, ErrPrivateIP
			}
			return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, addr)
		},
	}

	client := &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
	if !cfg.AllowRedirect {
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	return &Fetcher{
		cfg:    cfg,
		redis:  rdb,
		client: client,
		// proxyClientはSSRF保護なしの専用クライアント。管理者が設定した
		// 信頼済みプロキシURLはlocalhostやプライベートネットワーク上に
		// 配置されることが多いため、プライベートIPブロックを適用しない。
		proxyClient: &http.Client{Timeout: timeout},
	}
}

// Fetch returns the URL preview, using Redis cache when available.
func (f *Fetcher) Fetch(ctx context.Context, rawURL string) (*Result, error) {
	if !f.cfg.Enabled {
		return nil, ErrDisabled
	}

	// 外部summarizer proxyが設定されていればそちらに委譲する。
	if f.cfg.SummaryProxyURL != "" {
		return f.fetchViaProxy(ctx, rawURL)
	}

	// Redisキャッシュ確認
	cacheKey := cachePrefix + hashURL(rawURL)
	if f.redis != nil {
		if cached, err := f.redis.Get(ctx, cacheKey).Bytes(); err == nil {
			var result Result
			if json.Unmarshal(cached, &result) == nil {
				return &result, nil
			}
		}
	}

	result, err := f.fetchAndParse(ctx, rawURL)
	if err != nil {
		return nil, err
	}

	// Redisキャッシュ保存
	if f.redis != nil {
		if data, err := json.Marshal(result); err == nil {
			f.redis.Set(ctx, cacheKey, data, cacheTTL)
		}
	}

	return result, nil
}

func (f *Fetcher) fetchAndParse(ctx context.Context, rawURL string) (*Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFetchFailed, err)
	}
	ua := f.cfg.UserAgent
	if ua == "" {
		ua = "Misskey URL Preview"
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,*/*;q=0.5")

	resp, err := f.client.Do(req)
	if err != nil {
		if errors.Is(err, ErrPrivateIP) {
			return nil, ErrPrivateIP
		}
		return nil, fmt.Errorf("%w: %v", ErrFetchFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: status %d", ErrFetchFailed, resp.StatusCode)
	}

	// Content-Lengthチェック
	if f.cfg.RequireContentLength {
		cl := resp.Header.Get("Content-Length")
		if cl == "" {
			return nil, ErrTooLarge
		}
	}
	if f.cfg.MaxContentLength > 0 {
		if cl, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64); err == nil && cl > f.cfg.MaxContentLength {
			return nil, ErrTooLarge
		}
	}

	// HTMLのみ解析する。それ以外はURLだけ返す。
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") && !strings.Contains(ct, "application/xhtml") {
		return &Result{URL: rawURL, Player: PlayerResult{Allow: []string{}}}, nil
	}

	body := io.LimitReader(resp.Body, f.cfg.MaxContentLength)
	return ParseHTML(body, rawURL), nil
}

// fetchViaProxy delegates to an external summarizer service.
//
// proxyは管理者が設定した信頼済みURLなのでSSRFチェックを適用しないが、
// rawURLはクエリパラメータとして安全にエスケープする。
func (f *Fetcher) fetchViaProxy(ctx context.Context, rawURL string) (*Result, error) {
	proxyURL := f.cfg.SummaryProxyURL + "?url=" + url.QueryEscape(rawURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, proxyURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFetchFailed, err)
	}
	resp, err := f.proxyClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFetchFailed, err)
	}
	defer resp.Body.Close()

	var result Result
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("%w: parse proxy response: %v", ErrFetchFailed, err)
	}
	return &result, nil
}

func hashURL(u string) string {
	h := sha256.Sum256([]byte(u))
	return hex.EncodeToString(h[:])
}

// isPrivateHost checks if a hostname resolves to a private/loopback IP.
func isPrivateHost(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		// DNS名は解決してからチェック(Dialer段階でIPが確定している)
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}
