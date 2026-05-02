package mediaproxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
		assert.Equal(t, want, IsVideoMIME(ct), "%q", ct)
	}
}

// TestProcessAndReturn_VideoFallback ensures a video MIME flowing through a
// resize-class mode produces the dummy PNG (lite build) rather than failing
// the request, so frontend renders a placeholder.
func TestProcessAndReturn_VideoFallback(t *testing.T) {
	s := testService(nil)
	res, err := s.processAndReturn([]byte("fake video bytes"), "video/mp4", ModePreview, FormatWebP)
	assert.NoError(t, err)
	defer res.Body.Close()
	// dummy PNG fallback
	assert.Equal(t, "image/png", res.ContentType)
}

// TestIsResizeMode covers the predicate used to gate video thumbnail logic.
func TestIsResizeMode(t *testing.T) {
	for _, m := range []ProxyMode{ModeEmoji, ModeAvatar, ModeStatic, ModePreview, ModeBadge} {
		assert.True(t, isResizeMode(m))
	}
	assert.False(t, isResizeMode(ModeDefault))
}
