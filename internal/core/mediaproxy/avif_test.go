package mediaproxy

import (
	"bytes"
	"image"
	"image/color"
	"testing"

	"github.com/gen2brain/avif"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeAVIF returns a freshly encoded AVIF image with the given dimensions so
// the decoder side can be exercised directly (#637 M3 / M4 / M5 で
// 入力対応した wazero decoder の sanity check)。
func makeAVIF(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.NRGBA{uint8(x), uint8(y), 60, 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, avif.Encode(&buf, img, avif.Options{Quality: 60, Speed: 10}))
	return buf.Bytes()
}

// TestProcessResize_AVIFOutput exercises FormatAVIF: we feed a regular PNG in
// and require the output to be AVIF (verified by re-decoding through gen2brain).
func TestProcessResize_AVIFOutput(t *testing.T) {
	s := testService(nil)
	src := makePNG()

	res, err := s.processResize(src, "image/png", 0, 80, FormatAVIF)
	require.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, "image/avif", res.ContentType)

	var buf bytes.Buffer
	_, err = buf.ReadFrom(res.Body)
	require.NoError(t, err)

	dec, err := avif.Decode(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	assert.Equal(t, 80, dec.Bounds().Dy())
}

// TestProcessResize_AVIFInput proves the wazero AVIF decoder is wired in:
// processResize accepts AVIF as the source and re-encodes to WebP successfully.
func TestProcessResize_AVIFInput(t *testing.T) {
	s := testService(nil)
	src := makeAVIF(t, 200, 150)

	res, err := s.processResize(src, "image/avif", 0, 80, FormatWebP)
	require.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, "image/webp", res.ContentType)
}

// TestIsConvertibleImage_NewMIMEs ensures the new MIME types added in M3-M5
// are flagged convertible so the resize path reaches the decoder rather than
// falling back to passThrough.
func TestIsConvertibleImage_NewMIMEs(t *testing.T) {
	for _, mt := range []string{"image/avif", "image/heic", "image/heif", "image/jxl"} {
		assert.True(t, isConvertibleImage(mt), "%s should be convertible", mt)
	}
}
