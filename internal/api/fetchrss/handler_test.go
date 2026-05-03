package fetchrss

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/mmcdole/gofeed"
	ext "github.com/mmcdole/gofeed/extensions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleRSS2 = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:dc="http://purl.org/dc/elements/1.1/">
<channel>
  <title>Example Feed</title>
  <link>https://example.com/</link>
  <description>An example feed</description>
  <image>
    <url>https://example.com/icon.png</url>
    <title>Example Icon</title>
  </image>
  <item>
    <title>First Post</title>
    <link>https://example.com/posts/1</link>
    <guid isPermaLink="false">post-1</guid>
    <pubDate>Fri, 01 May 2026 12:00:00 GMT</pubDate>
    <dc:creator>Alice</dc:creator>
    <category>tech</category>
    <category>golang</category>
    <description>&lt;p&gt;Hello   world&lt;/p&gt;</description>
    <enclosure url="https://example.com/audio.mp3" length="1234" type="audio/mpeg"/>
  </item>
</channel>
</rss>`

const sampleAtom = `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Atom Sample</title>
  <link href="https://atom.example.com/"/>
  <updated>2026-05-01T12:00:00Z</updated>
  <id>urn:uuid:atom-feed</id>
  <entry>
    <title>Atom Entry</title>
    <link href="https://atom.example.com/e/1"/>
    <id>urn:uuid:atom-1</id>
    <updated>2026-05-01T12:00:00Z</updated>
    <author><name>Bob</name></author>
    <content type="html">&lt;p&gt;Atom &lt;b&gt;content&lt;/b&gt;&lt;/p&gt;</content>
  </entry>
</feed>`

// startFeedServer launches an httptest server returning the supplied body and
// returns a Handler whose HTTP client targets that server. We bypass SSRF
// concerns at the transport layer because httptest binds to 127.0.0.1; the
// real wiring uses the SSRF-safe transport from server/outbound_http.go.
func newTestHandler(_ *testing.T, srv *httptest.Server) *Handler {
	return New(&http.Client{Timeout: FetchTimeout, Transport: srv.Client().Transport})
}

func newRequestCtx(method, target string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(method, target, nil)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	return c, rec
}

func TestFetchRSS_BasicRSS2(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.Header.Get("Accept"), "application/rss+xml")
		w.Header().Set("Content-Type", "application/rss+xml")
		fmt.Fprint(w, sampleRSS2)
	}))
	t.Cleanup(srv.Close)

	h := newTestHandler(t, srv)
	c, rec := newRequestCtx(http.MethodGet, "/api/fetch-rss?url="+url.QueryEscape(srv.URL))
	require.NoError(t, h.Fetch(c))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	assert.Equal(t, "Example Feed", resp["title"])
	assert.Equal(t, "https://example.com/", resp["link"])
	assert.Equal(t, "An example feed", resp["description"])

	img, ok := resp["image"].(map[string]any)
	require.True(t, ok, "image must be present")
	assert.Equal(t, "https://example.com/icon.png", img["url"])
	assert.Equal(t, "Example Icon", img["title"])

	items, ok := resp["items"].([]any)
	require.True(t, ok, "items must be array")
	require.Len(t, items, 1)
	item := items[0].(map[string]any)
	assert.Equal(t, "First Post", item["title"])
	assert.Equal(t, "https://example.com/posts/1", item["link"])
	assert.Equal(t, "post-1", item["guid"])
	assert.Equal(t, "Alice", item["creator"])
	assert.Equal(t, "Fri, 01 May 2026 12:00:00 GMT", item["pubDate"])

	// isoDate は RFC3339 UTC で生 pubDate と一致する時刻に正規化される
	iso, ok := item["isoDate"].(string)
	require.True(t, ok)
	parsed, err := time.Parse(time.RFC3339Nano, iso)
	require.NoError(t, err)
	assert.True(t, parsed.UTC().Equal(time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)))

	cats, _ := item["categories"].([]any)
	assert.Equal(t, []any{"tech", "golang"}, cats)

	encl, ok := item["enclosure"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "https://example.com/audio.mp3", encl["url"])
	assert.Equal(t, "audio/mpeg", encl["type"])
	// length は Misskey schema では number。json.Unmarshal は float64 になる
	assert.EqualValues(t, 1234, encl["length"])

	// contentSnippet は description の HTML タグ除去 + 連続 whitespace 縮約
	assert.Equal(t, "Hello world", item["contentSnippet"])

	assert.Equal(t, "public, max-age=180", rec.Header().Get("Cache-Control"))
}

func TestFetchRSS_Atom(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		fmt.Fprint(w, sampleAtom)
	}))
	t.Cleanup(srv.Close)

	h := newTestHandler(t, srv)
	c, rec := newRequestCtx(http.MethodGet, "/api/fetch-rss?url="+url.QueryEscape(srv.URL))
	require.NoError(t, h.Fetch(c))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "Atom Sample", resp["title"])
	items := resp["items"].([]any)
	require.Len(t, items, 1)
	item := items[0].(map[string]any)
	assert.Equal(t, "Atom Entry", item["title"])
	assert.Equal(t, "Bob", item["creator"])
	assert.Equal(t, "Atom content", item["contentSnippet"])

	// Atom は published を持たないので isoDate は updated から派生する
	iso, ok := item["isoDate"].(string)
	require.True(t, ok)
	_, err := time.Parse(time.RFC3339Nano, iso)
	require.NoError(t, err)
}

func TestFetchRSS_POSTBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, sampleRSS2)
	}))
	t.Cleanup(srv.Close)

	h := newTestHandler(t, srv)
	body := fmt.Sprintf(`{"url":%q}`, srv.URL)
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/fetch-rss", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, h.Fetch(c))
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestFetchRSS_MissingURL(t *testing.T) {
	h := New(&http.Client{Timeout: FetchTimeout})
	c, rec := newRequestCtx(http.MethodGet, "/api/fetch-rss")
	require.NoError(t, h.Fetch(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	errObj := resp["error"].(map[string]any)
	assert.Equal(t, "INVALID_URL", errObj["code"])
}

func TestFetchRSS_InvalidScheme(t *testing.T) {
	h := New(&http.Client{Timeout: FetchTimeout})
	c, rec := newRequestCtx(http.MethodGet, "/api/fetch-rss?url=file:///etc/passwd")
	require.NoError(t, h.Fetch(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	errObj := resp["error"].(map[string]any)
	assert.Equal(t, "INVALID_URL", errObj["code"])
}

func TestFetchRSS_EmptyURLAfterTrim(t *testing.T) {
	h := New(&http.Client{Timeout: FetchTimeout})
	c, rec := newRequestCtx(http.MethodGet, "/api/fetch-rss?url=%20%20")
	require.NoError(t, h.Fetch(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestFetchRSS_NoHostInURL(t *testing.T) {
	h := New(&http.Client{Timeout: FetchTimeout})
	c, rec := newRequestCtx(http.MethodGet, "/api/fetch-rss?url=http://")
	require.NoError(t, h.Fetch(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestFetchRSS_UpstreamErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	h := newTestHandler(t, srv)
	c, rec := newRequestCtx(http.MethodGet, "/api/fetch-rss?url="+url.QueryEscape(srv.URL))
	require.NoError(t, h.Fetch(c))
	assert.Equal(t, http.StatusBadGateway, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	errObj := resp["error"].(map[string]any)
	assert.Equal(t, "UPSTREAM_ERROR", errObj["code"])
}

func TestFetchRSS_UnparseableBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "not a valid rss or atom")
	}))
	t.Cleanup(srv.Close)

	h := newTestHandler(t, srv)
	c, rec := newRequestCtx(http.MethodGet, "/api/fetch-rss?url="+url.QueryEscape(srv.URL))
	require.NoError(t, h.Fetch(c))
	assert.Equal(t, http.StatusBadGateway, rec.Code)
}

func TestFetchRSS_OversizedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Cap is 1 MiB; send 2 MiB. ReadAllLimit returns ErrMaxBytesExceeded so
		// fetchFeed surfaces UPSTREAM_ERROR before we ever call gofeed.
		w.Header().Set("Content-Length", fmt.Sprintf("%d", 2<<20))
		w.Write(make([]byte, 2<<20))
	}))
	t.Cleanup(srv.Close)

	h := newTestHandler(t, srv)
	c, rec := newRequestCtx(http.MethodGet, "/api/fetch-rss?url="+url.QueryEscape(srv.URL))
	require.NoError(t, h.Fetch(c))
	assert.Equal(t, http.StatusBadGateway, rec.Code)
}

// TestFetchRSS_DialFails covers the case where the outbound transport itself
// rejects the connection (which is how SSRF blocks surface in production).
func TestFetchRSS_DialFails(t *testing.T) {
	failingClient := &http.Client{
		Timeout: FetchTimeout,
		Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
			return nil, errors.New("ssrf block: connection to private IP refused")
		}),
	}
	h := New(failingClient)
	c, rec := newRequestCtx(http.MethodGet, "/api/fetch-rss?url=http://10.0.0.1/feed.xml")
	require.NoError(t, h.Fetch(c))
	assert.Equal(t, http.StatusBadGateway, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	errObj := resp["error"].(map[string]any)
	assert.Equal(t, "UPSTREAM_ERROR", errObj["code"])
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// --- pack helper unit tests (carve out edge cases that don't surface from
// the full RSS / Atom samples) ---

func TestPackImage_Nil(t *testing.T) {
	assert.Nil(t, packImage(nil))
}

func TestPackEnclosure_BadLength(t *testing.T) {
	// length が parse できない値の場合は length フィールドだけ省略し、url/type は残す
	out := packEnclosure([]*gofeed.Enclosure{{
		URL:    "https://example.com/audio.mp3",
		Type:   "audio/mpeg",
		Length: "not-a-number",
	}})
	require.NotNil(t, out)
	assert.Equal(t, "https://example.com/audio.mp3", out["url"])
	assert.Equal(t, "audio/mpeg", out["type"])
	_, hasLen := out["length"]
	assert.False(t, hasLen, "length must be omitted when not numeric")
}

func TestPackEnclosure_Empty(t *testing.T) {
	assert.Nil(t, packEnclosure(nil))
	assert.Nil(t, packEnclosure([]*gofeed.Enclosure{nil}))
	assert.Nil(t, packEnclosure([]*gofeed.Enclosure{{}}))
}

func TestPackImage_TitleOnly(t *testing.T) {
	// URL 空の Image (本来は schema 違反だが防御的に省略する)
	assert.Nil(t, packImage(&gofeed.Image{Title: "lonely"}))
}

func TestPackImage_NoTitle(t *testing.T) {
	out := packImage(&gofeed.Image{URL: "https://example.com/i.png"})
	require.NotNil(t, out)
	assert.Equal(t, "https://example.com/i.png", out["url"])
	_, hasTitle := out["title"]
	assert.False(t, hasTitle)
}

func TestPackITunes_Nil(t *testing.T) {
	assert.Nil(t, packITunes(&gofeed.Feed{}))
	// extension 全フィールド空 → 結果も nil
	assert.Nil(t, packITunes(&gofeed.Feed{ITunesExt: &ext.ITunesFeedExtension{}}))
}

func TestPackITunes_Full(t *testing.T) {
	feed := &gofeed.Feed{ITunesExt: &ext.ITunesFeedExtension{
		Image:    "https://example.com/cover.jpg",
		Author:   "Show Host",
		Summary:  "All about feeds",
		Explicit: "no",
		Keywords: "tech, golang , , rss",
		Owner: &ext.ITunesOwner{
			Name:  "Owner Name",
			Email: "owner@example.com",
		},
		Categories: []*ext.ITunesCategory{
			{Text: "Technology", Subcategory: &ext.ITunesCategory{Text: "Programming"}},
			{Text: "News"},
			nil,
			{Text: ""},
		},
	}}
	out := packITunes(feed)
	require.NotNil(t, out)
	assert.Equal(t, "https://example.com/cover.jpg", out["image"])
	assert.Equal(t, "Show Host", out["author"])
	assert.Equal(t, "All about feeds", out["summary"])
	assert.Equal(t, "no", out["explicit"])
	assert.Equal(t, []string{"tech", "golang", "rss"}, out["keywords"])
	assert.Equal(t, []string{"Technology", "Programming", "News"}, out["categories"])
	owner := out["owner"].(map[string]any)
	assert.Equal(t, "Owner Name", owner["name"])
	assert.Equal(t, "owner@example.com", owner["email"])
}

func TestPackITunes_KeywordsAllBlank(t *testing.T) {
	out := packITunes(&gofeed.Feed{ITunesExt: &ext.ITunesFeedExtension{
		Keywords: ", ,  ,",
	}})
	// blank-only keywords は categories と同じく省略する
	assert.Nil(t, out)
}

func TestPackITunes_OwnerNameOnly(t *testing.T) {
	out := packITunes(&gofeed.Feed{ITunesExt: &ext.ITunesFeedExtension{
		Owner: &ext.ITunesOwner{Name: "Solo"},
	}})
	require.NotNil(t, out)
	owner := out["owner"].(map[string]any)
	assert.Equal(t, "Solo", owner["name"])
	_, hasEmail := owner["email"]
	assert.False(t, hasEmail)
}

func TestItemCreator_Authors(t *testing.T) {
	got := itemCreator(&gofeed.Item{
		Authors: []*gofeed.Person{{Name: "Atom Author"}},
		Author:  &gofeed.Person{Name: "Deprecated"},
	})
	assert.Equal(t, "Atom Author", got)
}

func TestItemCreator_DeprecatedAuthor(t *testing.T) {
	got := itemCreator(&gofeed.Item{
		Author: &gofeed.Person{Name: "Old Author"},
	})
	assert.Equal(t, "Old Author", got)
}

func TestItemCreator_DublinCoreFallback(t *testing.T) {
	got := itemCreator(&gofeed.Item{
		DublinCoreExt: &ext.DublinCoreExtension{Creator: []string{"DC Creator"}},
	})
	assert.Equal(t, "DC Creator", got)
}

func TestItemCreator_Empty(t *testing.T) {
	assert.Equal(t, "", itemCreator(&gofeed.Item{}))
	// nil authors / empty arrays
	assert.Equal(t, "", itemCreator(&gofeed.Item{Authors: []*gofeed.Person{nil}}))
	assert.Equal(t, "", itemCreator(&gofeed.Item{Authors: []*gofeed.Person{{}}}))
}

func TestContentSnippet_Empty(t *testing.T) {
	assert.Equal(t, "", contentSnippet(&gofeed.Item{}))
}

func TestContentSnippet_DescriptionFallback(t *testing.T) {
	got := contentSnippet(&gofeed.Item{Description: "<b>Hello</b>"})
	assert.Equal(t, "Hello", got)
}

func TestPackItem_PublishedFallback(t *testing.T) {
	// PublishedParsed が無く UpdatedParsed しか無い場合 isoDate は updated 由来
	updated := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	out := packItem(&gofeed.Item{
		Title:         "x",
		UpdatedParsed: &updated,
	})
	iso, ok := out["isoDate"].(string)
	require.True(t, ok)
	parsed, err := time.Parse(time.RFC3339Nano, iso)
	require.NoError(t, err)
	assert.True(t, parsed.UTC().Equal(updated))
}
