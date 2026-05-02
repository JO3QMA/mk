//go:build !ffmpeg

package mediaproxy

import "errors"

// ErrVideoThumbnailUnavailable is returned by extractVideoThumbnailFrame when
// the binary was built without the `ffmpeg` build tag.
var ErrVideoThumbnailUnavailable = errors.New("mediaproxy: video thumbnail extraction not available (rebuild with FEATURES=ffmpeg)")

// extractVideoThumbnailFrame is a no-op in the lite build. The proxy falls
// back to its dummy PNG when this returns the sentinel error so frontend
// rendering keeps degrading gracefully instead of failing the whole
// request (#637 M2).
func extractVideoThumbnailFrame(_ []byte, _ string) ([]byte, error) {
	return nil, ErrVideoThumbnailUnavailable
}
