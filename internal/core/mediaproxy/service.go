package mediaproxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/chai2010/webp"
	"github.com/kovidgoyal/imaging"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	coredrive "github.com/shiroha-a/mk/internal/core/drive"
)

// ProxyMode enumerates the image processing modes.
type ProxyMode int

const (
	ModeDefault ProxyMode = iota
	ModeEmoji
	ModeAvatar
	ModeStatic
	ModePreview
	ModeBadge
)

// 画像処理パラメータ (Misskey TS準拠)
const (
	emojiHeight   = 128
	avatarHeight  = 320
	staticWidth   = 498
	staticHeight  = 422
	previewWidth  = 200
	previewHeight = 200
	badgeSize     = 96
	webpQuality   = 77
	maxDownload   = 32 << 20 // 32 MB
)

var (
	ErrUnauthorized = errors.New("mediaproxy: unauthorized URL")
	ErrNotFound     = errors.New("mediaproxy: resource not found")
	ErrBadRequest   = errors.New("mediaproxy: bad request")
	ErrTooLarge     = errors.New("mediaproxy: file too large")
)

// browsersafeMIMEs lists MIME types safe to serve inline in browsers.
var browsersafeMIMEs = map[string]bool{
	"image/png":              true,
	"image/gif":              true,
	"image/jpeg":             true,
	"image/webp":             true,
	"image/avif":             true,
	"image/apng":             true,
	"image/bmp":              true,
	"image/tiff":             true,
	"image/x-icon":           true,
	"image/vnd.mozilla.apng": true,
	"audio/opus":             true,
	"video/ogg":              true,
	"audio/ogg":              true,
	"application/ogg":        true,
	"video/quicktime":        true,
	"video/mp4":              true,
	"audio/mp4":              true,
	"video/x-m4v":            true,
	"audio/x-m4a":            true,
	"video/3gpp":             true,
	"video/3gpp2":            true,
	"video/mpeg":             true,
	"audio/mpeg":             true,
	"video/webm":             true,
	"audio/webm":             true,
	"audio/aac":              true,
	"audio/flac":             true,
	"audio/wav":              true,
}

// ProxyResult is the output of proxy resolution + processing.
type ProxyResult struct {
	Body        io.ReadCloser
	ContentType string
}

// Service handles media proxy authorization and fetching.
type Service struct {
	instanceURL  string
	driveStorage coredrive.Storage
	allowlist    AllowlistChecker
	hmacSecret   []byte
	httpClient   *http.Client
	userAgent    string
}

// NewService creates a new media proxy Service.
// allowedPrivateNetworks は SSRF 保護で許可するプライベート CIDR リスト (config.AllowedPrivateNetworks)。
func NewService(instanceURL, userAgent string, driveStorage coredrive.Storage, allowlist AllowlistChecker, hmacSecret []byte, allowedPrivateNetworks []string) *Service {
	return &Service{
		instanceURL:  instanceURL,
		driveStorage: driveStorage,
		allowlist:    allowlist,
		hmacSecret:   hmacSecret,
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: NewSSRFSafeTransport(allowedPrivateNetworks),
		},
		userAgent: userAgent,
	}
}

// SignURL generates an HMAC-SHA256 signature for the given URL.
func (s *Service) SignURL(rawURL string) string {
	return SignURL(s.hmacSecret, rawURL)
}

// Authorize checks whether the given URL is permitted for proxying.
// HMAC署名があれば先に検証し、なければDBのallowlistを参照する。
func (s *Service) Authorize(ctx context.Context, rawURL, sig string) error {
	if sig != "" && VerifyHMAC(s.hmacSecret, rawURL, sig) {
		return nil
	}

	allowed, err := s.allowlist.IsAllowedURL(ctx, rawURL)
	if err != nil {
		slog.Error("allowlist check failed", "url", rawURL, "error", err)
		return ErrUnauthorized
	}
	if !allowed {
		return ErrUnauthorized
	}
	return nil
}

// Fetch downloads the remote URL (or resolves a local file), applies image
// processing per the requested mode, and returns the result.
func (s *Service) Fetch(ctx context.Context, rawURL string, mode ProxyMode) (*ProxyResult, error) {
	// ローカルファイルの場合はdriveStorageから直接取得
	filesPrefix := s.instanceURL + "/files/"
	if strings.HasPrefix(rawURL, filesPrefix) {
		return s.resolveLocal(rawURL, filesPrefix, mode)
	}

	return s.fetchRemote(ctx, rawURL, mode)
}

// resolveLocal fetches a file from local drive storage by access key.
func (s *Service) resolveLocal(rawURL, filesPrefix string, mode ProxyMode) (*ProxyResult, error) {
	accessKey := strings.TrimPrefix(rawURL, filesPrefix)
	// パスに/が含まれる場合は先頭のセグメントだけを使う
	if idx := strings.Index(accessKey, "/"); idx >= 0 {
		accessKey = accessKey[:idx]
	}
	if accessKey == "" {
		return nil, ErrBadRequest
	}

	body, err := s.driveStorage.Get(accessKey)
	if err != nil {
		if errors.Is(err, coredrive.ErrObjectNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("mediaproxy: local file access: %w", err)
	}

	// ローカルファイルの場合はMIME判定して画像処理
	data, readErr := io.ReadAll(io.LimitReader(body, maxDownload+1))
	body.Close()
	if readErr != nil {
		return nil, fmt.Errorf("mediaproxy: read local file: %w", readErr)
	}
	if int64(len(data)) > maxDownload {
		return nil, ErrTooLarge
	}

	contentType := http.DetectContentType(data)
	return s.processAndReturn(data, contentType, mode)
}

// fetchRemote downloads a file from a remote URL.
func (s *Service) fetchRemote(ctx context.Context, rawURL string, mode ProxyMode) (*ProxyResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("mediaproxy: create request: %w", err)
	}
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "image/*,*/*;q=0.5")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, ErrNotFound
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		return nil, ErrNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("mediaproxy: remote returned %d", resp.StatusCode)
	}

	// Content-Lengthが明示されていてmaxDownloadを超える場合は即拒否
	if resp.ContentLength > maxDownload {
		return nil, ErrTooLarge
	}

	// maxDownload+1バイト読み、実際にmaxDownloadを超えたらエラー
	// Content-Lengthが嘘や未設定の場合でもサイレント切り捨てを防ぐ
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxDownload+1))
	if err != nil {
		return nil, fmt.Errorf("mediaproxy: read remote: %w", err)
	}
	if int64(len(data)) > maxDownload {
		return nil, ErrTooLarge
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = http.DetectContentType(data)
	}

	return s.processAndReturn(data, contentType, mode)
}

// processAndReturn applies image processing per mode and returns the result.
func (s *Service) processAndReturn(data []byte, contentType string, mode ProxyMode) (*ProxyResult, error) {
	switch mode {
	case ModeEmoji:
		return s.processResize(data, contentType, 0, emojiHeight)
	case ModeAvatar:
		return s.processResize(data, contentType, 0, avatarHeight)
	case ModeStatic:
		return s.processResize(data, contentType, staticWidth, staticHeight)
	case ModePreview:
		return s.processResize(data, contentType, previewWidth, previewHeight)
	case ModeBadge:
		return s.processBadge(data, contentType)
	default:
		return s.passThrough(data, contentType)
	}
}

// processResize decodes the image, resizes it, and encodes to WebP.
// width=0の場合はheightのみでアスペクト比を維持する。
func (s *Service) processResize(data []byte, contentType string, width, height int) (*ProxyResult, error) {
	if !isConvertibleImage(contentType) {
		// 変換できない画像フォーマットはそのまま返す
		return makeResult(data, contentType), nil
	}

	img, err := decodeImage(data)
	if err != nil {
		// デコード失敗時は元データをそのまま返す
		return makeResult(data, contentType), nil
	}

	var resized image.Image
	if width == 0 {
		// height指定のみ: アスペクト比を維持して高さに合わせる
		resized = resizeToHeight(img, height)
	} else {
		resized = resizeFit(img, width, height)
	}

	encoded, err := encodeWebP(resized)
	if err != nil {
		return makeResult(data, contentType), nil
	}
	return makeResult(encoded, "image/webp"), nil
}

// processBadge creates a 96x96 greyscale PNG badge.
func (s *Service) processBadge(data []byte, contentType string) (*ProxyResult, error) {
	if !isConvertibleImage(contentType) {
		return makeResult(data, contentType), nil
	}

	img, err := decodeImage(data)
	if err != nil {
		return makeResult(data, contentType), nil
	}

	// 96x96にリサイズしてグレースケール変換
	resized := imaging.Fill(img, badgeSize, badgeSize, imaging.Center, imaging.Lanczos)
	grey := imaging.Grayscale(resized)

	var buf bytes.Buffer
	if err := png.Encode(&buf, grey); err != nil {
		return makeResult(data, contentType), nil
	}
	return makeResult(buf.Bytes(), "image/png"), nil
}

// passThrough validates the MIME type and returns data as-is.
func (s *Service) passThrough(data []byte, contentType string) (*ProxyResult, error) {
	// image/svg+xmlはセキュリティ上そのまま返さない（XSSリスク）
	if contentType == "image/svg+xml" {
		return s.svgFallback(data)
	}

	if !browsersafeMIMEs[contentType] {
		return nil, fmt.Errorf("mediaproxy: rejected MIME type: %s", contentType)
	}

	return makeResult(data, contentType), nil
}

// svgFallback converts SVG to a 1x1 transparent PNG placeholder.
// SVGラスタライズはv2で実装予定。
func (s *Service) svgFallback(_ []byte) (*ProxyResult, error) {
	return makeDummyPNG(), nil
}

// DummyPNG returns a 1x1 transparent PNG for fallback responses.
func DummyPNG() *ProxyResult {
	return makeDummyPNG()
}

func makeDummyPNG() *ProxyResult {
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.Transparent)
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return makeResult(buf.Bytes(), "image/png")
}

func makeResult(data []byte, contentType string) *ProxyResult {
	return &ProxyResult{
		Body:        io.NopCloser(bytes.NewReader(data)),
		ContentType: contentType,
	}
}

// isConvertibleImage returns true if the MIME type can be decoded and converted.
func isConvertibleImage(mime string) bool {
	switch mime {
	case "image/jpeg", "image/png", "image/gif",
		"image/webp", "image/bmp", "image/tiff",
		"image/x-icon", "image/vnd.mozilla.apng":
		return true
	default:
		return false
	}
}

func decodeImage(data []byte) (image.Image, error) {
	img, err := imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
	if err != nil {
		return nil, err
	}
	return img, nil
}

// resizeToHeight resizes image to the specified height while preserving aspect
// ratio. 元画像がheight以下の場合は拡大し��い。
func resizeToHeight(img image.Image, height int) image.Image {
	bounds := img.Bounds()
	if bounds.Dy() <= height {
		return img
	}
	return imaging.Resize(img, 0, height, imaging.Lanczos)
}

// resizeFit resizes img to fit within maxW x maxH preserving aspect ratio.
func resizeFit(img image.Image, maxW, maxH int) image.Image {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= maxW && h <= maxH {
		return img
	}
	return imaging.Fit(img, maxW, maxH, imaging.Lanczos)
}

func encodeWebP(img image.Image) ([]byte, error) {
	// webp.Encode はNRGBA型を期待する
	bounds := img.Bounds()
	nrgba := image.NewNRGBA(bounds)
	draw.Draw(nrgba, bounds, img, bounds.Min, draw.Src)

	var buf bytes.Buffer
	if err := webp.Encode(&buf, nrgba, &webp.Options{Quality: float32(webpQuality)}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
