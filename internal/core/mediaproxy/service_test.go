package mediaproxy

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mock AllowlistChecker ---

type mockAllowlist struct {
	allowed map[string]bool
}

func (m *mockAllowlist) IsAllowedURL(_ context.Context, url string) (bool, error) {
	return m.allowed[url], nil
}

// --- mock Storage ---

type mockStorage struct {
	files map[string][]byte
}

func (m *mockStorage) Put(_ string, _ io.Reader) (string, error) { return "", nil }
func (m *mockStorage) Delete(_ string) error                     { return nil }
func (m *mockStorage) Get(key string) (io.ReadCloser, error) {
	data, ok := m.files[key]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return io.NopCloser(io.Reader(io.NopCloser(
		&byteReadCloser{data: data, pos: 0},
	))), nil
}

type byteReadCloser struct {
	data []byte
	pos  int
}

func (b *byteReadCloser) Read(p []byte) (int, error) {
	if b.pos >= len(b.data) {
		return 0, io.EOF
	}
	n := copy(p, b.data[b.pos:])
	b.pos += n
	return n, nil
}
func (b *byteReadCloser) Close() error { return nil }

// --- test helpers ---

func testService(allowedURLs map[string]bool) *Service {
	return NewService(
		"https://example.com",
		"Misskey/2026.3.2 (https://example.com)",
		&mockStorage{files: map[string][]byte{}},
		&mockAllowlist{allowed: allowedURLs},
		[]byte("test-secret"),
	)
}

func makePNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	w := &byteWriter{}
	_ = png.Encode(w, img)
	return w.data
}

type byteWriter struct {
	data []byte
}

func (w *byteWriter) Write(p []byte) (int, error) {
	w.data = append(w.data, p...)
	return len(p), nil
}

// --- tests ---

func TestAuthorize_ValidHMAC(t *testing.T) {
	s := testService(nil)
	url := "https://remote.example/avatar.png"
	sig := s.SignURL(url)

	err := s.Authorize(context.Background(), url, sig)
	assert.NoError(t, err)
}

func TestAuthorize_InvalidHMAC_AllowlistedURL(t *testing.T) {
	s := testService(map[string]bool{
		"https://remote.example/avatar.png": true,
	})

	err := s.Authorize(context.Background(), "https://remote.example/avatar.png", "wrong-sig")
	assert.NoError(t, err)
}

func TestAuthorize_NoSig_AllowlistedURL(t *testing.T) {
	s := testService(map[string]bool{
		"https://remote.example/avatar.png": true,
	})

	err := s.Authorize(context.Background(), "https://remote.example/avatar.png", "")
	assert.NoError(t, err)
}

func TestAuthorize_NoSig_NotAllowlisted(t *testing.T) {
	s := testService(map[string]bool{})

	err := s.Authorize(context.Background(), "https://evil.example/malware.exe", "")
	assert.ErrorIs(t, err, ErrUnauthorized)
}

func TestAuthorize_InvalidHMAC_NotAllowlisted(t *testing.T) {
	s := testService(map[string]bool{})

	err := s.Authorize(context.Background(), "https://evil.example/malware.exe", "bad-sig")
	assert.ErrorIs(t, err, ErrUnauthorized)
}

func TestFetch_RemoteImage(t *testing.T) {
	imgData := makePNG()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(imgData)
	}))
	defer ts.Close()

	s := testService(map[string]bool{ts.URL + "/img.png": true})

	result, err := s.Fetch(context.Background(), ts.URL+"/img.png", ModeDefault)
	require.NoError(t, err)
	defer result.Body.Close()

	assert.Equal(t, "image/png", result.ContentType)

	body, _ := io.ReadAll(result.Body)
	assert.NotEmpty(t, body)
}

func TestFetch_RemoteImage_Emoji(t *testing.T) {
	imgData := makePNG()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(imgData)
	}))
	defer ts.Close()

	s := testService(map[string]bool{ts.URL + "/emoji.png": true})

	result, err := s.Fetch(context.Background(), ts.URL+"/emoji.png", ModeEmoji)
	require.NoError(t, err)
	defer result.Body.Close()

	assert.Equal(t, "image/webp", result.ContentType)
}

func TestFetch_RemoteImage_Avatar(t *testing.T) {
	imgData := makePNG()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(imgData)
	}))
	defer ts.Close()

	s := testService(map[string]bool{ts.URL + "/avatar.png": true})

	result, err := s.Fetch(context.Background(), ts.URL+"/avatar.png", ModeAvatar)
	require.NoError(t, err)
	defer result.Body.Close()

	assert.Equal(t, "image/webp", result.ContentType)
}

func TestFetch_RemoteImage_Static(t *testing.T) {
	imgData := makePNG()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(imgData)
	}))
	defer ts.Close()

	s := testService(map[string]bool{ts.URL + "/img.png": true})

	result, err := s.Fetch(context.Background(), ts.URL+"/img.png", ModeStatic)
	require.NoError(t, err)
	defer result.Body.Close()

	assert.Equal(t, "image/webp", result.ContentType)
}

func TestFetch_RemoteImage_Preview(t *testing.T) {
	imgData := makePNG()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(imgData)
	}))
	defer ts.Close()

	s := testService(map[string]bool{ts.URL + "/img.png": true})

	result, err := s.Fetch(context.Background(), ts.URL+"/img.png", ModePreview)
	require.NoError(t, err)
	defer result.Body.Close()

	assert.Equal(t, "image/webp", result.ContentType)
}

func TestFetch_RemoteImage_Badge(t *testing.T) {
	imgData := makePNG()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(imgData)
	}))
	defer ts.Close()

	s := testService(map[string]bool{ts.URL + "/img.png": true})

	result, err := s.Fetch(context.Background(), ts.URL+"/img.png", ModeBadge)
	require.NoError(t, err)
	defer result.Body.Close()

	assert.Equal(t, "image/png", result.ContentType)
}

func TestFetch_Remote404(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	s := testService(map[string]bool{ts.URL + "/missing.png": true})

	_, err := s.Fetch(context.Background(), ts.URL+"/missing.png", ModeDefault)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestFetch_LocalFile(t *testing.T) {
	imgData := makePNG()

	s := NewService(
		"https://example.com",
		"Misskey/2026.3.2 (https://example.com)",
		&mockStorage{files: map[string][]byte{"abc123": imgData}},
		&mockAllowlist{allowed: map[string]bool{}},
		[]byte("test-secret"),
	)

	result, err := s.Fetch(context.Background(), "https://example.com/files/abc123", ModeDefault)
	require.NoError(t, err)
	defer result.Body.Close()

	assert.Equal(t, "image/png", result.ContentType)
}

func TestFetch_UnsafeMIME_Rejected(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Write([]byte("alert('xss')"))
	}))
	defer ts.Close()

	s := testService(map[string]bool{ts.URL + "/evil.js": true})

	_, err := s.Fetch(context.Background(), ts.URL+"/evil.js", ModeDefault)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rejected MIME type")
}

func TestDummyPNG(t *testing.T) {
	result := DummyPNG()
	defer result.Body.Close()
	assert.Equal(t, "image/png", result.ContentType)
	data, _ := io.ReadAll(result.Body)
	assert.NotEmpty(t, data)
}

func TestIsConvertibleImage(t *testing.T) {
	tests := []struct {
		mime string
		want bool
	}{
		{"image/png", true},
		{"image/jpeg", true},
		{"image/gif", true},
		{"image/webp", true},
		{"image/bmp", true},
		{"image/tiff", true},
		{"image/svg+xml", false},
		{"video/mp4", false},
		{"text/plain", false},
	}
	for _, tt := range tests {
		t.Run(tt.mime, func(t *testing.T) {
			assert.Equal(t, tt.want, isConvertibleImage(tt.mime))
		})
	}
}

func TestBrowsersafeMIMEs(t *testing.T) {
	assert.True(t, browsersafeMIMEs["image/png"])
	assert.True(t, browsersafeMIMEs["video/mp4"])
	assert.True(t, browsersafeMIMEs["audio/mpeg"])
	assert.False(t, browsersafeMIMEs["application/javascript"])
	assert.False(t, browsersafeMIMEs["text/html"])
}

func TestFetch_SVG_ReturnsDummyPNG(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg"><rect width="10" height="10"/></svg>`))
	}))
	defer ts.Close()

	s := testService(map[string]bool{ts.URL + "/icon.svg": true})

	result, err := s.Fetch(context.Background(), ts.URL+"/icon.svg", ModeDefault)
	require.NoError(t, err)
	defer result.Body.Close()

	// SVGはXSSリスクがあるためダミーPNGにフォールバック
	assert.Equal(t, "image/png", result.ContentType)
}

func TestFetch_LocalFile_NotFound(t *testing.T) {
	s := NewService(
		"https://example.com",
		"Misskey/2026.3.2 (https://example.com)",
		&mockStorage{files: map[string][]byte{}},
		&mockAllowlist{allowed: map[string]bool{}},
		[]byte("test-secret"),
	)

	_, err := s.Fetch(context.Background(), "https://example.com/files/nonexistent", ModeDefault)
	assert.Error(t, err)
}

func TestFetch_LocalFile_EmptyAccessKey(t *testing.T) {
	s := NewService(
		"https://example.com",
		"Misskey/2026.3.2 (https://example.com)",
		&mockStorage{files: map[string][]byte{}},
		&mockAllowlist{allowed: map[string]bool{}},
		[]byte("test-secret"),
	)

	_, err := s.Fetch(context.Background(), "https://example.com/files/", ModeDefault)
	assert.ErrorIs(t, err, ErrBadRequest)
}

func TestFetch_LocalFile_WithPathSegments(t *testing.T) {
	imgData := makePNG()
	s := NewService(
		"https://example.com",
		"Misskey/2026.3.2 (https://example.com)",
		&mockStorage{files: map[string][]byte{"abc123": imgData}},
		&mockAllowlist{allowed: map[string]bool{}},
		[]byte("test-secret"),
	)

	// /files/abc123/extra のようなパスでもabc123だけ使う
	result, err := s.Fetch(context.Background(), "https://example.com/files/abc123/extra", ModeDefault)
	require.NoError(t, err)
	defer result.Body.Close()
	assert.Equal(t, "image/png", result.ContentType)
}

func TestFetch_LocalFile_Emoji(t *testing.T) {
	imgData := makePNG()
	s := NewService(
		"https://example.com",
		"Misskey/2026.3.2 (https://example.com)",
		&mockStorage{files: map[string][]byte{"emoji1": imgData}},
		&mockAllowlist{allowed: map[string]bool{}},
		[]byte("test-secret"),
	)

	result, err := s.Fetch(context.Background(), "https://example.com/files/emoji1", ModeEmoji)
	require.NoError(t, err)
	defer result.Body.Close()
	assert.Equal(t, "image/webp", result.ContentType)
}

func TestFetch_RemoteServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	s := testService(map[string]bool{ts.URL + "/error.png": true})

	_, err := s.Fetch(context.Background(), ts.URL+"/error.png", ModeDefault)
	assert.Error(t, err)
}

func TestFetch_RemoteGone(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusGone)
	}))
	defer ts.Close()

	s := testService(map[string]bool{ts.URL + "/deleted.png": true})

	_, err := s.Fetch(context.Background(), ts.URL+"/deleted.png", ModeDefault)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestFetch_RemoteNoContentType(t *testing.T) {
	imgData := makePNG()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Content-Typeヘッダなしで返す
		w.Write(imgData)
	}))
	defer ts.Close()

	s := testService(map[string]bool{ts.URL + "/img.png": true})

	result, err := s.Fetch(context.Background(), ts.URL+"/img.png", ModeDefault)
	require.NoError(t, err)
	defer result.Body.Close()
	// auto-detected from content
	assert.Equal(t, "image/png", result.ContentType)
}

func TestProcessResize_NonConvertibleImage(t *testing.T) {
	s := testService(nil)
	data := []byte("not an image")
	result, err := s.processResize(data, "application/octet-stream", 100, 100)
	require.NoError(t, err)
	defer result.Body.Close()
	// 変換できない場合はそのまま返す
	assert.Equal(t, "application/octet-stream", result.ContentType)
}

func TestProcessBadge_NonConvertibleImage(t *testing.T) {
	s := testService(nil)
	data := []byte("not an image")
	result, err := s.processBadge(data, "text/plain")
	require.NoError(t, err)
	defer result.Body.Close()
	assert.Equal(t, "text/plain", result.ContentType)
}

func TestResizeToHeight_SmallImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 50, 50))
	result := resizeToHeight(img, 128)
	// 元画像が128以下なので拡大されない
	assert.Equal(t, 50, result.Bounds().Dy())
}

func TestResizeFit_SmallImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 50, 50))
	result := resizeFit(img, 498, 422)
	// 元画像が範囲内なのでそのまま
	assert.Equal(t, 50, result.Bounds().Dx())
	assert.Equal(t, 50, result.Bounds().Dy())
}

func TestDecodeImage_InvalidData(t *testing.T) {
	_, err := decodeImage([]byte("not an image"))
	assert.Error(t, err)
}

func TestEncodeWebP(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	data, err := encodeWebP(img)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
}

func TestSignURL_MatchesStandalone(t *testing.T) {
	s := testService(nil)
	url := "https://example.com/test.png"
	sig := s.SignURL(url)
	assert.Equal(t, SignURL([]byte("test-secret"), url), sig)
}

func TestResizeToHeight_LargeImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 500, 500))
	result := resizeToHeight(img, 128)
	assert.Equal(t, 128, result.Bounds().Dy())
}

func TestResizeFit_LargeImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1000, 1000))
	result := resizeFit(img, 200, 200)
	assert.LessOrEqual(t, result.Bounds().Dx(), 200)
	assert.LessOrEqual(t, result.Bounds().Dy(), 200)
}

func TestFetch_Remote_InvalidURL(t *testing.T) {
	s := testService(map[string]bool{"not://valid": true})

	_, err := s.Fetch(context.Background(), "not://valid", ModeDefault)
	assert.Error(t, err)
}

func TestProcessResize_LargeImage_WidthAndHeight(t *testing.T) {
	s := testService(nil)
	imgData := makePNG() // 100x100

	result, err := s.processResize(imgData, "image/png", 50, 50)
	require.NoError(t, err)
	defer result.Body.Close()
	assert.Equal(t, "image/webp", result.ContentType)
}

func TestProcessResize_HeightOnly(t *testing.T) {
	s := testService(nil)
	imgData := makePNG() // 100x100

	result, err := s.processResize(imgData, "image/png", 0, 50)
	require.NoError(t, err)
	defer result.Body.Close()
	assert.Equal(t, "image/webp", result.ContentType)
}

func TestProcessBadge_ValidImage(t *testing.T) {
	s := testService(nil)
	imgData := makePNG() // 100x100

	result, err := s.processBadge(imgData, "image/png")
	require.NoError(t, err)
	defer result.Body.Close()
	assert.Equal(t, "image/png", result.ContentType)
}

func TestAuthorize_AllowlistError(t *testing.T) {
	// errorAllowlistはIsAllowedURLでエラーを返す
	s := NewService(
		"https://example.com",
		"Misskey/2026.3.2 (https://example.com)",
		&mockStorage{files: map[string][]byte{}},
		&errorAllowlist{},
		[]byte("test-secret"),
	)

	err := s.Authorize(context.Background(), "https://example.com/img.png", "")
	assert.ErrorIs(t, err, ErrUnauthorized)
}

type errorAllowlist struct{}

func (e *errorAllowlist) IsAllowedURL(_ context.Context, _ string) (bool, error) {
	return false, fmt.Errorf("db connection failed")
}
