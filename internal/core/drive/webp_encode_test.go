package drive

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"testing"

	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// 追加のテスト用画像ヘルパ
// 既存の image_processor_test.go では JPEG/PNG/GIF のみカバーしているため、
// ここで BMP / TIFF / WebP / 透過 PNG を補う。差し替え予定の WebP encoder
// 経路 (encodeWebP) を multiple format input × quality × size で固める。
// ---------------------------------------------------------------------------

// makeTestBMP creates a small BMP image with a deterministic gradient.
func makeTestBMP(w, h int) []byte {
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

// makeTestTIFF creates a small TIFF image with a deterministic gradient.
func makeTestTIFF(w, h int) []byte {
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

// makeTestWebP creates a small WebP image via the production encoder.
// 差し替え後はこの helper も新しいエンコーダを通るので、テストデータ生成側と
// 検証側がずれる心配がない (production と同じ経路を踏む)。
func makeTestWebP(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{120, uint8(x), uint8(y), 255})
		}
	}
	data, err := encodeWebP(img, webpQuality)
	require.NoError(t, err)
	return data
}

// makeTransparentPNG creates an RGBA PNG with the top-left quadrant fully
// transparent and the bottom-right opaque red. alpha 保持の round-trip 検証用。
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

// decodeWebPForTest decodes WebP bytes via the pure Go decoder registered
// through `_ "golang.org/x/image/webp"`. encoder の差し替えに引きずられない。
func decodeWebPForTest(t *testing.T, data []byte) image.Image {
	t.Helper()
	img, _, err := image.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	return img
}

// ---------------------------------------------------------------------------
// Round-trip と入力フォーマット網羅
// ---------------------------------------------------------------------------

// TestEncodeWebP_RoundTrip_SolidColor verifies that encoding a solid-color
// image and decoding the result yields the same dimensions and a near-equal
// center pixel. 色空間ハンドリングが壊れた場合に検出する。
func TestEncodeWebP_RoundTrip_SolidColor(t *testing.T) {
	const w, h = 100, 80
	src := image.NewRGBA(image.Rect(0, 0, w, h))
	red := color.RGBA{220, 30, 50, 255}
	draw.Draw(src, src.Bounds(), &image.Uniform{C: red}, image.Point{}, draw.Src)

	data, err := encodeWebP(src, webpQuality)
	require.NoError(t, err)

	dec := decodeWebPForTest(t, data)
	assert.Equal(t, w, dec.Bounds().Dx())
	assert.Equal(t, h, dec.Bounds().Dy())

	// solid color の中央 1 ピクセルを lossy 誤差の許容幅で比較する。
	r, g, b, _ := dec.At(w/2, h/2).RGBA()
	const tol = 8.0
	assert.InDelta(t, int(red.R), int(r>>8), tol)
	assert.InDelta(t, int(red.G), int(g>>8), tol)
	assert.InDelta(t, int(red.B), int(b>>8), tol)
}

// TestEncodeWebP_AllInputFormats_Thumbnail exercises GenerateThumbnail with
// JPEG/PNG/GIF/BMP/TIFF/WebP inputs to ensure each decodes and the resulting
// WebP is itself decodable. WebP encoder 差し替え時に format ごとの挙動差を
// 検出する目的。
func TestEncodeWebP_AllInputFormats_Thumbnail(t *testing.T) {
	cases := []struct {
		name string
		mime string
		body []byte
	}{
		{"jpeg", "image/jpeg", makeTestJPEG(200, 150)},
		{"png", "image/png", makeTestPNG(200, 150)},
		{"gif", "image/gif", makeTestGIF(200, 150)},
		{"bmp", "image/bmp", makeTestBMP(200, 150)},
		{"tiff", "image/tiff", makeTestTIFF(200, 150)},
		{"webp", "image/webp", makeTestWebP(t, 200, 150)},
	}
	proc := NewDefaultImageProcessor()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := proc.GenerateThumbnail(tc.body, tc.mime)
			require.NoError(t, err)
			require.NotNil(t, result, "GenerateThumbnail returned nil for %s", tc.mime)
			assert.Equal(t, "image/webp", result.MimeType)

			dec := decodeWebPForTest(t, result.Data)
			assert.LessOrEqual(t, dec.Bounds().Dx(), thumbnailWidth)
			assert.LessOrEqual(t, dec.Bounds().Dy(), thumbnailHeight)
			assert.Positive(t, dec.Bounds().Dx())
			assert.Positive(t, dec.Bounds().Dy())
		})
	}
}

// TestEncodeWebP_AlphaPreserved verifies that fully transparent regions in
// the source survive the encode round-trip. chai2010 → gen2brain swap で
// alpha チャネルが落ちると失敗する。
func TestEncodeWebP_AlphaPreserved(t *testing.T) {
	const w, h = 80, 80
	src, _, err := image.Decode(bytes.NewReader(makeTransparentPNG(w, h)))
	require.NoError(t, err)

	data, err := encodeWebP(src, webpQuality)
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
// size-specific encoder bugs (1x1, 奇数寸法, 細長, 大サイズ)。
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
			data, err := encodeWebP(img, webpQuality)
			require.NoError(t, err)
			dec := decodeWebPForTest(t, data)
			assert.Equal(t, tc.w, dec.Bounds().Dx())
			assert.Equal(t, tc.h, dec.Bounds().Dy())
		})
	}
}

// TestEncodeWebP_QualityRange encodes the same varied image at multiple
// quality levels and asserts that q=10 produces strictly smaller output
// than q=95. quality scaling の方向 (or value range mismatch) を検出する。
// gen2brain は Quality=100 を lossless 扱いする差分があるため、ここでは
// 95 までを比較対象にする。
func TestEncodeWebP_QualityRange(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 256, 256))
	for y := range 256 {
		for x := range 256 {
			src.Set(x, y, color.RGBA{uint8(x), uint8(y), uint8((x + y) & 0xFF), 255})
		}
	}
	qualities := []int{10, 50, 77, 95}
	sizes := make(map[int]int, len(qualities))
	for _, q := range qualities {
		data, err := encodeWebP(src, q)
		require.NoError(t, err, "encode at q=%d", q)
		dec := decodeWebPForTest(t, data)
		assert.Equal(t, 256, dec.Bounds().Dx(), "q=%d", q)
		assert.Equal(t, 256, dec.Bounds().Dy(), "q=%d", q)
		sizes[q] = len(data)
	}
	assert.Less(t, sizes[10], sizes[95], "q=10 size should be < q=95 size: %v", sizes)
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

// BenchmarkEncodeWebP encodes a representative 800x600 gradient image at
// production quality (77). chai2010 を baseline として記録し、Step 2 で
// gen2brain との比較指標にする。
func BenchmarkEncodeWebP(b *testing.B) {
	img := image.NewRGBA(image.Rect(0, 0, 800, 600))
	for y := range 600 {
		for x := range 800 {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), uint8((x + y) & 0xFF), 255})
		}
	}
	b.ResetTimer()
	for range b.N {
		if _, err := encodeWebP(img, webpQuality); err != nil {
			b.Fatal(err)
		}
	}
}
