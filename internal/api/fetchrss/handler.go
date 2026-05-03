// Package fetchrss implements the /api/fetch-rss endpoint, used by the
// RSS / RSSTicker widgets on the Misskey frontend. Fetches a remote RSS or
// Atom feed and translates it into the Misskey-compatible response shape
// produced by the upstream TS implementation (which wraps the rss-parser
// npm package).
package fetchrss

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/mmcdole/gofeed"
	"github.com/shiroha-a/mk/internal/api/apierr"
	"github.com/shiroha-a/mk/internal/safehttp"
)

// CacheSeconds matches the upstream Misskey `cacheSec: 60 * 3` declaration so
// the frontend's browser cache and any reverse-proxy caches behave the same
// against mk-go and the original TS backend.
const CacheSeconds = 60 * 3

// MaxBodyBytes caps the response read from the upstream feed server. Real-world
// feeds rarely exceed a few hundred KB; capping at 1 MiB blocks pathological
// hosts that try to drown the parser in arbitrary bytes.
const MaxBodyBytes int64 = 1 << 20

// FetchTimeout matches the upstream `timeout: 5000` ms used by Misskey TS.
const FetchTimeout = 5 * time.Second

// Handler serves /api/fetch-rss. The HTTP client must be wired with an
// SSRF-safe transport (see internal/server/outbound_http.go); the handler
// itself adds only request-shape validation and gofeed translation. Note
// that safehttp.NewSSRFSafeTransport applies the private-IP guard on every
// dial, so HTTP redirects to private IPs are also blocked — the handler
// can rely on the transport for redirect-chain SSRF defense.
type Handler struct {
	httpClient *http.Client
	userAgent  string
	parserPool *sync.Pool
}

// New builds a Handler bound to the given outbound HTTP client and User-Agent
// string. Pass a client whose Transport is safehttp.NewSSRFSafeTransport(...)
// so private IPs and non-http(s) schemes never leak through this endpoint.
// userAgent should normally be cfg.UserAgent so feed servers see the same
// `Misskey-Go/<ver>` identification used everywhere else.
func New(httpClient *http.Client, userAgent string) *Handler {
	return &Handler{
		httpClient: httpClient,
		userAgent:  userAgent,
		// gofeed.Parser の goroutine 安全性は明記されていないため pool 経由で
		// 1 リクエスト 1 インスタンスに分離する。Pool reuse でアロケーションは
		// 実質ゼロに保つ。
		parserPool: &sync.Pool{New: func() any { return gofeed.NewParser() }},
	}
}

// Fetch handles GET/POST /api/fetch-rss. The frontend RSS widget uses GET with
// a query string, so we accept both shapes.
func (h *Handler) Fetch(c echo.Context) error {
	rawURL := strings.TrimSpace(c.QueryParam("url"))
	if rawURL == "" {
		var body struct {
			URL string `json:"url"`
		}
		// Bind 失敗は無視 (空 URL のままバリデーションで弾く)。Misskey TS は
		// JSON Schema で require するが、こちらでは下の guard で同等の
		// 400 を返す。
		_ = c.Bind(&body)
		rawURL = strings.TrimSpace(body.URL)
	}

	if rawURL == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_URL", "url is required.", "9c5ad7d3-6e15-4f3a-87b8-39ec2e91d5a3"))
	}

	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		// 不正 scheme / host 欠落と「URL 未指定」を frontend 側で別扱いに
		// したい運用に備え、Misskey の慣行どおり別 ID を割り当てる。
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_URL", "url must be http(s).", "f5b2bd41-7c0a-4d49-b8c8-3d3a4d9b8e21"))
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), FetchTimeout)
	defer cancel()

	feed, err := h.fetchFeed(ctx, u.String())
	if err != nil {
		// SSRF block / dial fail / parse fail はすべて upstream 側の問題として
		// 502 にまとめる。err.Error() を直接 client に返すと resolved IP や
		// SSRF 文言が漏れるため static メッセージに置き換え、詳細はサーバ側
		// ログに残す。frontend は items の有無しか見ていないので、stub と
		// 同じく空 array を返す道もあるが、ウィジェット側で「取得失敗」と
		// 「フィードが空」を区別したい運用に備えて explicit error にする。
		slog.WarnContext(ctx, "fetch-rss upstream failed", "url", u.String(), "err", err)
		return c.JSON(http.StatusBadGateway, apierr.Error("UPSTREAM_ERROR", "Failed to fetch feed.", "0e0e1f5b-2c97-4f17-a51c-1f9c2e9b4d82"))
	}

	c.Response().Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", CacheSeconds))
	return c.JSON(http.StatusOK, packFeed(feed))
}

// fetchFeed performs the GET + body cap + gofeed parse. Kept separate so
// tests can stub at a coarser layer.
func (h *Handler) fetchFeed(ctx context.Context, feedURL string) (*gofeed.Feed, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	// Misskey TS は `Accept: application/rss+xml, */*` を送る。Atom 専用 server
	// 互換のため */* も付ける。
	req.Header.Set("Accept", "application/rss+xml, */*")
	if h.userAgent != "" {
		// UA 必須の RSS 配信サーバ (Cloudflare 含む) で 403 にならないように
		// するため、他の outbound 経路と同じ Misskey-Go/<ver> UA を送る。
		req.Header.Set("User-Agent", h.userAgent)
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("upstream status %d", resp.StatusCode)
	}

	body, err := safehttp.ReadAllLimit(resp.Body, MaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	parser := h.parserPool.Get().(*gofeed.Parser)
	defer h.parserPool.Put(parser)

	feed, err := parser.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse feed: %w", err)
	}
	if feed == nil {
		return nil, errors.New("parser returned nil feed")
	}
	return feed, nil
}

// packFeed maps gofeed.Feed onto the Misskey-compatible response shape. Empty
// fields are omitted entirely (Misskey TS marks them `optional: true` and
// rss-parser leaves them undefined). `items` is always present as an array,
// matching the `optional: false` declaration in the upstream meta.
func packFeed(feed *gofeed.Feed) map[string]any {
	out := map[string]any{}
	if feed.Title != "" {
		out["title"] = feed.Title
	}
	if feed.Description != "" {
		out["description"] = feed.Description
	}
	if feed.Link != "" {
		out["link"] = feed.Link
	}
	if feed.FeedLink != "" {
		out["feedUrl"] = feed.FeedLink
	}
	if img := packImage(feed.Image); img != nil {
		out["image"] = img
	}
	if itunes := packITunes(feed); itunes != nil {
		out["itunes"] = itunes
	}

	items := make([]map[string]any, 0, len(feed.Items))
	for _, it := range feed.Items {
		items = append(items, packItem(it))
	}
	out["items"] = items

	return out
}

func packImage(img *gofeed.Image) map[string]any {
	if img == nil || img.URL == "" {
		return nil
	}
	out := map[string]any{"url": img.URL}
	if img.Title != "" {
		out["title"] = img.Title
	}
	return out
}

func packITunes(feed *gofeed.Feed) map[string]any {
	ext := feed.ITunesExt
	if ext == nil {
		return nil
	}
	out := map[string]any{}
	if ext.Image != "" {
		out["image"] = ext.Image
	}
	if ext.Author != "" {
		out["author"] = ext.Author
	}
	if ext.Summary != "" {
		out["summary"] = ext.Summary
	}
	if ext.Explicit != "" {
		out["explicit"] = ext.Explicit
	}
	if len(ext.Categories) > 0 {
		// gofeed の Categories は階層 ([{Text, Subcategory}]) だが、Misskey は
		// flat string[] を期待するので flatten する。
		cats := make([]string, 0, len(ext.Categories))
		for _, cat := range ext.Categories {
			if cat == nil || cat.Text == "" {
				continue
			}
			cats = append(cats, cat.Text)
			if cat.Subcategory != nil && cat.Subcategory.Text != "" {
				cats = append(cats, cat.Subcategory.Text)
			}
		}
		if len(cats) > 0 {
			out["categories"] = cats
		}
	}
	if ext.Keywords != "" {
		// gofeed は keywords を生のまま string で持つ (RSS itunes は CSV)。
		// Misskey schema は array なので rss-parser 互換に split + trim する。
		parts := strings.Split(ext.Keywords, ",")
		kws := make([]string, 0, len(parts))
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				kws = append(kws, s)
			}
		}
		if len(kws) > 0 {
			out["keywords"] = kws
		}
	}
	if ext.Owner != nil {
		owner := map[string]any{}
		if ext.Owner.Name != "" {
			owner["name"] = ext.Owner.Name
		}
		if ext.Owner.Email != "" {
			owner["email"] = ext.Owner.Email
		}
		if len(owner) > 0 {
			out["owner"] = owner
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func packItem(it *gofeed.Item) map[string]any {
	out := map[string]any{}
	if it.Title != "" {
		out["title"] = it.Title
	}
	if it.Link != "" {
		out["link"] = it.Link
	}
	if it.GUID != "" {
		out["guid"] = it.GUID
	}
	if it.Published != "" {
		out["pubDate"] = it.Published
	}
	if t := it.PublishedParsed; t != nil {
		out["isoDate"] = t.UTC().Format(time.RFC3339Nano)
	} else if t := it.UpdatedParsed; t != nil {
		// Atom feed では published より updated しか持たないことが多い。
		// rss-parser も isoDate fallback として updated を使う。
		out["isoDate"] = t.UTC().Format(time.RFC3339Nano)
	}
	if creator := itemCreator(it); creator != "" {
		out["creator"] = creator
	}
	if it.Description != "" {
		out["summary"] = it.Description
	}
	if it.Content != "" {
		out["content"] = it.Content
	}
	if snippet := contentSnippet(it); snippet != "" {
		out["contentSnippet"] = snippet
	}
	if len(it.Categories) > 0 {
		out["categories"] = it.Categories
	}
	if encl := packEnclosure(it.Enclosures); encl != nil {
		out["enclosure"] = encl
	}
	return out
}

func itemCreator(it *gofeed.Item) string {
	// Atom は <author><name>...</name></author>、RSS 2.0 は <dc:creator>...</dc:creator>
	// が一般的。両系統を rss-parser と同じ優先順で見る。
	if len(it.Authors) > 0 && it.Authors[0] != nil && it.Authors[0].Name != "" {
		return it.Authors[0].Name
	}
	if it.Author != nil && it.Author.Name != "" {
		return it.Author.Name
	}
	if it.DublinCoreExt != nil && len(it.DublinCoreExt.Creator) > 0 {
		return it.DublinCoreExt.Creator[0]
	}
	return ""
}

func packEnclosure(encs []*gofeed.Enclosure) map[string]any {
	if len(encs) == 0 {
		return nil
	}
	e := encs[0]
	if e == nil || e.URL == "" {
		return nil
	}
	out := map[string]any{"url": e.URL}
	if e.Type != "" {
		out["type"] = e.Type
	}
	if e.Length != "" {
		// rss-parser は length を Number に変換するため schema は number。
		// gofeed は string で持つので int 変換、失敗時は省略する。
		if n, err := strconv.ParseInt(e.Length, 10, 64); err == nil {
			out["length"] = n
		}
	}
	return out
}

var (
	htmlTagRE  = regexp.MustCompile(`<[^>]*>`)
	whitespace = regexp.MustCompile(`\s+`)
)

// contentSnippet mirrors rss-parser の contentSnippet 算出: content または
// description から HTML タグを除去し、whitespace を 1 個に正規化、前後 trim。
// frontend MFM レンダラに直接渡されることはないが、ticker のテキスト表示で
// 使われる場合があるため形だけ揃える。
func contentSnippet(it *gofeed.Item) string {
	src := it.Content
	if src == "" {
		src = it.Description
	}
	if src == "" {
		return ""
	}
	stripped := htmlTagRE.ReplaceAllString(src, "")
	stripped = whitespace.ReplaceAllString(stripped, " ")
	return strings.TrimSpace(stripped)
}
