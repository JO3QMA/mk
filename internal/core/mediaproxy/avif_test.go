package mediaproxy

import (
	"bytes"
	"image"
	"image/color"
	"testing"

	"github.com/gen2brain/avif"
	"github.com/gen2brain/jpegxl"
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

// makeJXL returns a JPEG XL-encoded image so the decoder side can be
// exercised end-to-end (#637 M5).
func makeJXL(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.NRGBA{uint8(x), uint8(y), 90, 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, jpegxl.Encode(&buf, img))
	return buf.Bytes()
}

// TestProcessResize_JPEGXLInput proves the wazero JPEG XL decoder is wired
// in: processResize accepts a real JPEG XL stream and re-encodes to WebP.
func TestProcessResize_JPEGXLInput(t *testing.T) {
	s := testService(nil)
	src := makeJXL(t, 200, 150)

	res, err := s.processResize(src, "image/jxl", 0, 80, FormatWebP)
	require.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, "image/webp", res.ContentType)
}

// TestProcessResize_PixelBombRejected verifies the pixel-bomb guard returns
// dummy PNG for an oversized decoded raster instead of allocating multi-GB
// resize buffers (#637 review UR-016).
func TestProcessResize_PixelBombRejected(t *testing.T) {
	// We can't realistically synthesize a 30000x30000 input here, but we
	// can shadow the limit for the test scope.
	prev := maxDecodedPixels
	maxDecodedPixels = 1024 // 32x32 cap
	defer func() { maxDecodedPixels = prev }()

	s := testService(nil)
	src := makePNG() // 100x100 = 10,000 px > 1024
	res, err := s.processResize(src, "image/png", 0, 80, FormatWebP)
	require.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, "image/png", res.ContentType, "must fall back to dummy PNG")
}

// TestProcessBadge_PixelBombRejected: same guard for the badge path.
func TestProcessBadge_PixelBombRejected(t *testing.T) {
	prev := maxDecodedPixels
	maxDecodedPixels = 1024
	defer func() { maxDecodedPixels = prev }()

	s := testService(nil)
	src := makePNG()
	res, err := s.processBadge(src, "image/png")
	require.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, "image/png", res.ContentType)
}

// TestExceedsPixelCap covers the pixel-bomb guard (#637 review UR-016).
func TestExceedsPixelCap(t *testing.T) {
	cases := []struct {
		name string
		w, h int
		want bool
	}{
		{"under cap (1080p)", 1920, 1080, false},
		{"at cap exact", 8192, 8192, false},
		{"over cap (large)", 30000, 30000, true},
		{"degenerate (0x0)", 0, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var img image.Image
			if tc.w <= 0 || tc.h <= 0 {
				// degenerate: empty rect that still exposes Dx()/Dy() = 0
				img = image.NewNRGBA(image.Rectangle{})
			} else {
				img = image.NewNRGBA(image.Rect(0, 0, tc.w, tc.h))
			}
			assert.Equal(t, tc.want, exceedsPixelCap(img))
		})
	}
}
