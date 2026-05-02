//go:build ffmpeg

package mediaproxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// videoThumbnailTimeout caps the per-extraction wall-clock budget. Rendering a
// single keyframe should complete well under this limit; longer than that we
// suspect a malformed input or stalled decoder.
const videoThumbnailTimeout = 30 * time.Second

// videoThumbnailMaxOutput caps the size of the encoded JPEG ffmpeg may
// produce. Slightly larger than the 200x200 preview need to leave headroom
// for higher-resolution sources.
const videoThumbnailMaxOutput = 4 << 20

// ErrVideoThumbnailUnavailable mirrors the stub-build sentinel so handler
// code can compare without caring about which build it links against.
var ErrVideoThumbnailUnavailable = errors.New("mediaproxy: video thumbnail extraction failed")

// extractVideoThumbnailFrame shells out to ffmpeg (`ffmpeg -i pipe:0 -frames:v 1 -f mjpeg pipe:1`)
// to grab the first decodable video frame as JPEG. The caller then feeds the
// JPEG into the standard image processing pipeline (resize / WebP / AVIF).
//
// 本実装は intentionally small — `os/exec` で ffmpeg を呼び出すだけで cgo
// dependency は持たない。ffmpeg バイナリが PATH に無い / decode 失敗 / output
// 上限超過は同じ ErrVideoThumbnailUnavailable に畳み込み、proxy 側で
// dummy PNG fallback に切り替える。
func extractVideoThumbnailFrame(body []byte, _ string) ([]byte, error) {
	if len(body) == 0 {
		return nil, ErrVideoThumbnailUnavailable
	}

	ctx, cancel := context.WithTimeout(context.Background(), videoThumbnailTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-loglevel", "error",
		"-i", "pipe:0",
		"-frames:v", "1",
		"-f", "mjpeg",
		"pipe:1",
	)
	cmd.Stdin = bytes.NewReader(body)
	var out bytes.Buffer
	cmd.Stdout = &limitWriter{W: &out, N: videoThumbnailMaxOutput}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%w: %v: %s", ErrVideoThumbnailUnavailable, err, stderr.String())
	}
	if out.Len() == 0 {
		return nil, ErrVideoThumbnailUnavailable
	}
	return out.Bytes(), nil
}

// limitWriter wraps io.Writer to short-circuit once N bytes have been
// written, surfacing a sentinel error so the ffmpeg process can be terminated
// before the buffer balloons.
type limitWriter struct {
	W io.Writer
	N int
}

func (l *limitWriter) Write(p []byte) (int, error) {
	if l.N <= 0 {
		return 0, errors.New("mediaproxy: ffmpeg output exceeded cap")
	}
	if len(p) > l.N {
		p = p[:l.N]
	}
	n, err := l.W.Write(p)
	l.N -= n
	return n, err
}
