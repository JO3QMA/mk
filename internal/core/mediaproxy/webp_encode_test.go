package mediaproxy

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"

	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// 入力フォーマット別ヘルパ
// 既存 service_test.go は makePNG しか持たないため、JPEG/GIF/BMP/TIFF/WebP/
// 透過 PNG をここで補い、WebP encoder 周辺を網羅できるようにする。
// ---------------------------------------------------------------------------

func makeJPEG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 90, 255})
		}
	}
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	return buf.Bytes()
}

func makeGIF(w, h int) []byte {
	img := image.NewPaletted(image.Rect(0, 0, w, h), color.Palette{
		color.RGBA{255, 0, 0, 255},
		color.RGBA{0, 255, 0, 255},
	})
	var buf bytes.Buffer
	_ = gif.Encode(&buf, img, nil)
	return buf.Bytes()
}

func makeBMP(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 80, 255})
		}
	}
	var buf bytes.Buffer
	_ = bmp.Encode(&buf, img)
	return buf.Bytes()
}

func makeTIFF(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x * 2), uint8(y * 2), 50, 255})
		}
	}
	var buf bytes.Buffer
	_ = tiff.Encode(&buf, img, nil)
	return buf.Bytes()
}

// makeWebP creates a small WebP image via the production encoder.
func makeWebP(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{120, uint8(x), uint8(y), 255})
		}
	}
	data, err := encodeWebP(img)
	require.NoError(t, err)
	return data
}

// makeTransparentPNG creates an RGBA PNG with the top-left quadrant fully
// transparent and the bottom-right opaque red.
func makeTransparentPNG(w, h int) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			if x < w/2 && y < h/2 {
				img.Set(x, y, color.NRGBA{0, 0, 0, 0})
			} else {
				img.Set(x, y, color.NRGBA{255, 0, 0, 255})
			}
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// decodeWebPForTest decodes WebP bytes via the registered pure Go decoder.
func decodeWebPForTest(t *testing.T, data []byte) image.Image {
	t.Helper()
	img, _, err := image.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	return img
}

// ---------------------------------------------------------------------------
// Round-trip と入力フォーマット網羅
// ---------------------------------------------------------------------------

// TestEncodeWebP_RoundTrip_SolidColor encodes a solid color image and
// verifies dimensions + center pixel are preserved within a tolerance.
// mediaproxy.encodeWebP は内部で NRGBA に正規化するため、その変換経路も
// 込みで検証する。
func TestEncodeWebP_RoundTrip_SolidColor(t *testing.T) {
	const w, h = 100, 80
	src := image.NewRGBA(image.Rect(0, 0, w, h))
	red := color.RGBA{220, 30, 50, 255}
	draw.Draw(src, src.Bounds(), &image.Uniform{C: red}, image.Point{}, draw.Src)

	data, err := encodeWebP(src)
	require.NoError(t, err)

	dec := decodeWebPForTest(t, data)
	assert.Equal(t, w, dec.Bounds().Dx())
	assert.Equal(t, h, dec.Bounds().Dy())

	r, g, b, _ := dec.At(w/2, h/2).RGBA()
	const tol = 8.0
	assert.InDelta(t, int(red.R), int(r>>8), tol)
	assert.InDelta(t, int(red.G), int(g>>8), tol)
	assert.InDelta(t, int(red.B), int(b>>8), tol)
}

// TestEncodeWebP_AllInputFormats_ProcessResize feeds JPEG/PNG/GIF/BMP/TIFF/
// WebP through processResize and verifies that the resulting bytes are valid
// WebP. WebP encoder swap 後も全ての入力フォーマットで成立することを保証する。
func TestEncodeWebP_AllInputFormats_ProcessResize(t *testing.T) {
	cases := []struct {
		name string
		mime string
		body []byte
	}{
		{"jpeg", "image/jpeg", makeJPEG(200, 150)},
		{"png", "image/png", makePNG()},
		{"gif", "image/gif", makeGIF(200, 150)},
		{"bmp", "image/bmp", makeBMP(200, 150)},
		{"tiff", "image/tiff", makeTIFF(200, 150)},
		{"webp", "image/webp", makeWebP(t, 200, 150)},
	}
	s := testService(nil)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := s.processResize(tc.body, tc.mime, 0, 80)
			require.NoError(t, err)
			defer result.Body.Close()
			assert.Equal(t, "image/webp", result.ContentType)

			// processResize の戻り値は Body 経由でしか中身が取れないので読み出す。
			buf := new(bytes.Buffer)
			_, err = buf.ReadFrom(result.Body)
			require.NoError(t, err)

			dec := decodeWebPForTest(t, buf.Bytes())
			assert.Equal(t, 80, dec.Bounds().Dy(), "%s: height should match resize target", tc.name)
			assert.Positive(t, dec.Bounds().Dx())
		})
	}
}

// TestEncodeWebP_AlphaPreserved encodes a PNG with a transparent quadrant
// and verifies alpha is preserved in the WebP output.
func TestEncodeWebP_AlphaPreserved(t *testing.T) {
	const w, h = 80, 80
	src, _, err := image.Decode(bytes.NewReader(makeTransparentPNG(w, h)))
	require.NoError(t, err)

	data, err := encodeWebP(src)
	require.NoError(t, err)

	dec := decodeWebPForTest(t, data)
	require.Equal(t, w, dec.Bounds().Dx())
	require.Equal(t, h, dec.Bounds().Dy())

	_, _, _, aT := dec.At(w/4, h/4).RGBA()
	_, _, _, aO := dec.At(3*w/4, 3*h/4).RGBA()
	assert.LessOrEqual(t, aT>>8, uint32(8), "transparent quadrant should stay near alpha=0")
	assert.GreaterOrEqual(t, aO>>8, uint32(240), "opaque quadrant should stay near alpha=255")
}

// TestEncodeWebP_EdgeSizes encodes images of various dimensions to detect
// size-specific encoder bugs.
func TestEncodeWebP_EdgeSizes(t *testing.T) {
	cases := []struct{ w, h int }{
		{1, 1},
		{3, 5},
		{17, 31},
		{1024, 8},
		{8, 1024},
		{2048, 2048},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%dx%d", tc.w, tc.h), func(t *testing.T) {
			img := image.NewRGBA(image.Rect(0, 0, tc.w, tc.h))
			data, err := encodeWebP(img)
			require.NoError(t, err)
			dec := decodeWebPForTest(t, data)
			assert.Equal(t, tc.w, dec.Bounds().Dx())
			assert.Equal(t, tc.h, dec.Bounds().Dy())
		})
	}
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

// BenchmarkEncodeWebP encodes a representative 800x600 gradient image at the
// fixed mediaproxy quality (77). chai2010 baseline 計測用。
func BenchmarkEncodeWebP(b *testing.B) {
	img := image.NewRGBA(image.Rect(0, 0, 800, 600))
	for y := range 600 {
		for x := range 800 {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), uint8((x + y) & 0xFF), 255})
		}
	}
	b.ResetTimer()
	for range b.N {
		if _, err := encodeWebP(img); err != nil {
			b.Fatal(err)
		}
	}
}
