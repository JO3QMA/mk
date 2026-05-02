//go:build !ffmpeg

package mediaproxy

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestExtractVideoThumbnailFrame_StubBuild verifies that without the ffmpeg
// build tag, extractVideoThumbnailFrame returns ErrVideoThumbnailUnavailable
// so the proxy falls back to the dummy PNG.
func TestExtractVideoThumbnailFrame_StubBuild(t *testing.T) {
	_, err := extractVideoThumbnailFrame([]byte("not a video"), "video/mp4")
	assert.True(t, errors.Is(err, ErrVideoThumbnailUnavailable),
		"lite build should signal video extraction unavailable")
}
