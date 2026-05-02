package mediaproxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIsVideoMIME covers the helper that gates whether the proxy attempts
// video thumbnail extraction.
func TestIsVideoMIME(t *testing.T) {
	cases := map[string]bool{
		"video/mp4":       true,
		"video/webm":      true,
		"VIDEO/MP4":       true, // 大文字混じりも video/* と認識
		"image/png":       false,
		"application/pdf": false,
		"":                false,
		"video":           false, // missing slash
		"audio/mp4":       false,
	}
	for ct, want := range cases {
		assert.Equal(t, want, isVideoMIME(ct), "%q", ct)
	}
}

// 設定無しの video request は dummy PNG fallback (frontend は placeholder
// 表示で degrade)。
func TestProcessAndReturn_VideoFallback_NoGenerator(t *testing.T) {
	s := testService(nil)
	res, err := s.processAndReturn(context.Background(), []byte("fake video bytes"), "video/mp4", ModePreview, FormatWebP, "https://remote.example/clip.mp4")
	assert.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, "image/png", res.ContentType)
}

// 外部 generator が設定されている場合、proxy は GET <gen>/thumbnail.webp?...
// に委譲する。返ってきた静止画は image pipeline (resize → WebP) を通る。
func TestProcessAndReturn_VideoGeneratorRoundtrip(t *testing.T) {
	gen := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/thumbnail.webp", r.URL.Path)
		assert.Equal(t, "1", r.URL.Query().Get("thumbnail"))
		assert.Equal(t, "https://remote.example/clip.mp4", r.URL.Query().Get("url"))
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(makePNG())
	}))
	defer gen.Close()

	s := testService(nil)
	s.SetVideoThumbnailGenerator(gen.URL)
	// httptest server の HTTP client を尊重させる (ssl 検証回避用 fake transport)
	s.videoThumbClient = gen.Client()

	res, err := s.processAndReturn(context.Background(), nil, "video/mp4", ModePreview, FormatWebP, "https://remote.example/clip.mp4")
	require.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, "image/webp", res.ContentType, "thumbnail must be re-encoded to webp")
}

// Generator が 5xx を返した場合、proxy は dummy PNG にフォールバックして
// frontend を壊さない。
func TestProcessAndReturn_VideoGeneratorFailure(t *testing.T) {
	gen := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer gen.Close()

	s := testService(nil)
	s.SetVideoThumbnailGenerator(gen.URL)
	s.videoThumbClient = gen.Client()

	res, err := s.processAndReturn(context.Background(), nil, "video/mp4", ModePreview, FormatWebP, "https://remote.example/clip.mp4")
	require.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, "image/png", res.ContentType)
}

// Local /files/ source は generator にループバックさせない (M1 swap 経路で
// 既に解決済の前提)。
func TestProcessAndReturn_VideoLocalSourceSkipsGenerator(t *testing.T) {
	gen := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("generator must not be called for local /files/ sources")
	}))
	defer gen.Close()

	s := testService(nil)
	s.SetVideoThumbnailGenerator(gen.URL)
	s.videoThumbClient = gen.Client()

	res, err := s.processAndReturn(context.Background(), nil, "video/mp4", ModePreview, FormatWebP, "https://example.com/files/abc123")
	require.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, "image/png", res.ContentType)
}

// videoThumbnailRequestURL builds the expected target for both the plain HTTP
// and Unix domain socket cases.
func TestVideoThumbnailRequestURL(t *testing.T) {
	t.Run("http", func(t *testing.T) {
		got, err := videoThumbnailRequestURL("https://thumb.example.com", "https://video.example.com/clip.mp4")
		require.NoError(t, err)
		u, err := url.Parse(got)
		require.NoError(t, err)
		assert.Equal(t, "thumb.example.com", u.Host)
		assert.Equal(t, "/thumbnail.webp", u.Path)
		assert.Equal(t, "https://video.example.com/clip.mp4", u.Query().Get("url"))
	})
	t.Run("unix", func(t *testing.T) {
		got, err := videoThumbnailRequestURL("unix:///run/video-thumb.sock", "https://video.example.com/clip.mp4")
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(got, "http://localhost/thumbnail.webp"))
		u, err := url.Parse(got)
		require.NoError(t, err)
		assert.Equal(t, "https://video.example.com/clip.mp4", u.Query().Get("url"))
	})
	t.Run("unix with stray query is ignored", func(t *testing.T) {
		got, err := videoThumbnailRequestURL("unix:///run/video-thumb.sock?token=xyz", "https://video.example.com/clip.mp4")
		require.NoError(t, err)
		// Stray query on the unix:// URL must NOT leak into the request URL
		// (regression for the TrimRight(s, "") dead-code from the previous
		// commit).
		assert.True(t, strings.HasPrefix(got, "http://localhost/thumbnail.webp"))
		u, err := url.Parse(got)
		require.NoError(t, err)
		assert.Equal(t, "", u.Query().Get("token"))
	})
}

// fetchVideoThumbnail surfaces ErrVideoThumbnailUnavailable when no client
// is configured.
func TestFetchVideoThumbnail_NoClient(t *testing.T) {
	s := testService(nil)
	_, _, err := s.fetchVideoThumbnail(context.Background(), "https://x/clip.mp4")
	assert.ErrorIs(t, err, ErrVideoThumbnailUnavailable)
}

// videoThumbnailClient: empty URL → nil client.
func TestNewVideoThumbnailClient_EmptyURL(t *testing.T) {
	assert.Nil(t, newVideoThumbnailClient(""))
}

// videoThumbnailClient: malformed unix:// URL → nil (no panic).
func TestNewVideoThumbnailClient_BadUnixURL(t *testing.T) {
	assert.Nil(t, newVideoThumbnailClient("unix://"))
}

// TestIsResizeMode covers the predicate used to gate video thumbnail logic.
func TestIsResizeMode(t *testing.T) {
	for _, m := range []ProxyMode{ModeEmoji, ModeAvatar, ModeStatic, ModePreview, ModeBadge} {
		assert.True(t, isResizeMode(m))
	}
	assert.False(t, isResizeMode(ModeDefault))
}
