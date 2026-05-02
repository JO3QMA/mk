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
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/gen2brain/avif"
	_ "github.com/gen2brain/heic" // HEIC/HEIF input decode (iPhone uploads)
	_ "github.com/gen2brain/jpegxl"
	"github.com/gen2brain/webp"
	"github.com/kovidgoyal/imaging"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	coredrive "github.com/shiroha-a/mk/internal/core/drive"
	"github.com/shiroha-a/mk/internal/safehttp"
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
	avifQuality   = 60       // AVIF default; matches gen2brain/avif.DefaultQuality
	avifSpeed     = 8        // 0..10 — 8 keeps quality close to default while staying responsive
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
	"image/png":  true,
	"image/gif":  true,
	"image/jpeg": true,
	"image/webp": true,
	"image/avif": true,
	"image/apng": true,
	"image/bmp":  true,
	"image/tiff": true,
	// HEIC/HEIF: Safari 16+ で表示可能。Firefox/Chrome は表示できないが
	// pass-through で返した先で frontend が WebP/AVIF 変換 URL に置換する
	// 想定 (#637 M4)。
	"image/heic": true,
	"image/heif": true,
	// JPEG XL: Safari 17+ 対応、Chrome は flag つきで実験中 (#637 M5)。
	"image/jxl": true,
	// `image/x-icon` は古い慣例、`image/vnd.microsoft.icon` は IANA media
	// type registry に登録されている公式名 (RFC 紐付け無し)。リモート
	// Misskey/Mastodon の favicon.ico は後者で返ってくるホストが多いため
	// 両方許可する (#418)。
	"image/x-icon":             true,
	"image/vnd.microsoft.icon": true,
	"image/vnd.mozilla.apng":   true,
	"audio/opus":               true,
	"video/ogg":                true,
	"audio/ogg":                true,
	"application/ogg":          true,
	"video/quicktime":          true,
	"video/mp4":                true,
	"audio/mp4":                true,
	"video/x-m4v":              true,
	"audio/x-m4a":              true,
	"video/3gpp":               true,
	"video/3gpp2":              true,
	"video/mpeg":               true,
	"audio/mpeg":               true,
	"video/webm":               true,
	"audio/webm":               true,
	"audio/aac":                true,
	"audio/flac":               true,
	"audio/wav":                true,
}

// ProxyResult is the output of proxy resolution + processing.
type ProxyResult struct {
	Body        io.ReadCloser
	ContentType string
}

// DriveFileLookup is the minimal subset of repository.DriveFileRepository
// the proxy needs to resolve a `/files/<accessKey>` request to its cached
// thumbnail / webpublic variant when one exists (#637 M1)。
type DriveFileLookup interface {
	FindByAccessKey(accessKey string) (DriveFileVariants, error)
}

// DriveFileVariants exposes only the access-key fields the proxy needs.
// repository 側でこの shape を直接返さない (model.DriveFile が大きすぎる)
// ので、wire 層で adapter を書いて変換する。
type DriveFileVariants struct {
	AccessKey          *string
	ThumbnailAccessKey *string
	WebpublicAccessKey *string
}

// Service handles media proxy authorization and fetching.
type Service struct {
	instanceURL  string
	driveStorage coredrive.Storage
	allowlist    AllowlistChecker
	hmacSecret   []byte
	httpClient   *http.Client
	userAgent    string
	driveLookup  DriveFileLookup // optional, #637 M1
}

// NewService creates a new media proxy Service.
// allowedPrivateNetworks は SSRF 保護で許可するプライベート CIDR リスト (config.AllowedPrivateNetworks)。
// transportOpts は forward proxy 等の追加 transport 設定 (safehttp.WithProxy など)。
func NewService(instanceURL, userAgent string, driveStorage coredrive.Storage, allowlist AllowlistChecker, hmacSecret []byte, allowedPrivateNetworks []string, transportOpts ...safehttp.Option) *Service {
	return &Service{
		instanceURL:  instanceURL,
		driveStorage: driveStorage,
		allowlist:    allowlist,
		hmacSecret:   hmacSecret,
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: NewSSRFSafeTransport(allowedPrivateNetworks, transportOpts...),
		},
		userAgent: userAgent,
	}
}

// SetDriveLookup attaches a DriveFileLookup so the proxy can substitute the
// cached thumbnail / webpublic variant for `?preview` and `?static` requests
// against local files (#637 M1)。
func (s *Service) SetDriveLookup(l DriveFileLookup) {
	s.driveLookup = l
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
// processing per the requested mode, and returns the result. out selects the
// encoder format (FormatWebP / FormatAVIF) for resize-class modes.
func (s *Service) Fetch(ctx context.Context, rawURL string, mode ProxyMode, out OutputFormat) (*ProxyResult, error) {
	// ローカルファイルの場合はdriveStorageから直接取得
	filesPrefix := s.instanceURL + "/files/"
	if strings.HasPrefix(rawURL, filesPrefix) {
		return s.resolveLocal(rawURL, filesPrefix, mode, out)
	}

	return s.fetchRemote(ctx, rawURL, mode, out)
}

// resolveLocal fetches a file from local drive storage by access key.
func (s *Service) resolveLocal(rawURL, filesPrefix string, mode ProxyMode, out OutputFormat) (*ProxyResult, error) {
	accessKey := strings.TrimPrefix(rawURL, filesPrefix)
	// パスに/が含まれる場合は先頭のセグメントだけを使う
	if idx := strings.Index(accessKey, "/"); idx >= 0 {
		accessKey = accessKey[:idx]
	}
	if accessKey == "" {
		return nil, ErrBadRequest
	}

	// 既に存在する thumbnail / webpublic variant を提供できる mode なら、
	// 元データを再 decode + resize せずに variant を直接返して CPU を節約
	// する (#637 M1)。SetDriveLookup されていない / 該当 variant が無い
	// 場合は従来通り元データを取得して proxy 側で resize する。
	if s.driveLookup != nil {
		if swapped, ok := s.swapToVariant(accessKey, mode); ok {
			accessKey = swapped
		}
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
	return s.processAndReturn(data, contentType, mode, out)
}

// swapToVariant looks up the DriveFile by access key and returns the
// thumbnail / webpublic access key when one matches the requested mode.
// Returns (newKey, true) on swap, ("", false) otherwise.
func (s *Service) swapToVariant(accessKey string, mode ProxyMode) (string, bool) {
	v, err := s.driveLookup.FindByAccessKey(accessKey)
	if err != nil {
		return "", false
	}
	// 既に primary 以外 (= 既に variant の access key) を要求されているなら
	// 二重に swap しない。
	if v.AccessKey == nil || *v.AccessKey != accessKey {
		return "", false
	}
	switch mode {
	case ModeEmoji, ModeAvatar, ModePreview, ModeBadge:
		// 小サイズ系: thumbnail があれば優先、無ければ webpublic を試す。
		if v.ThumbnailAccessKey != nil && *v.ThumbnailAccessKey != "" {
			return *v.ThumbnailAccessKey, true
		}
		if v.WebpublicAccessKey != nil && *v.WebpublicAccessKey != "" {
			return *v.WebpublicAccessKey, true
		}
	case ModeStatic:
		// 中サイズ: webpublic 優先、無ければ thumbnail を流用 (静止 + 縮小)。
		if v.WebpublicAccessKey != nil && *v.WebpublicAccessKey != "" {
			return *v.WebpublicAccessKey, true
		}
		if v.ThumbnailAccessKey != nil && *v.ThumbnailAccessKey != "" {
			return *v.ThumbnailAccessKey, true
		}
	}
	return "", false
}

// fetchRemote downloads a file from a remote URL.
func (s *Service) fetchRemote(ctx context.Context, rawURL string, mode ProxyMode, out OutputFormat) (*ProxyResult, error) {
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

	// Content-Type ヘッダーには `; charset=utf-8` 等のパラメータが付くことが
	// あるので、media type だけを切り出してから allowlist と比較する
	// (#418 Devin review)。`mime.ParseMediaType` 失敗時は元のヘッダ値を
	// そのまま使い、後段の DetectContentType フォールバックに任せる。
	rawCT := resp.Header.Get("Content-Type")
	contentType := rawCT
	if mt, _, err := mime.ParseMediaType(rawCT); err == nil {
		contentType = mt
	}
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = http.DetectContentType(data)
	}

	return s.processAndReturn(data, contentType, mode, out)
}

// OutputFormat selects the encoder used by resize-class processing modes.
type OutputFormat int

const (
	// FormatWebP is the default browser-safe encoder.
	FormatWebP OutputFormat = iota
	// FormatAVIF requests AVIF output. caller-side preconditions: client
	// either set ?avif=1 or sent `Accept: image/avif,...`. AVIF は CPU
	// 重めなので明示要求時のみ使う (Misskey TS と同 semantics)。
	FormatAVIF
)

// processAndReturn applies image processing per mode and returns the result
// in the requested output format (WebP / AVIF). Badge と passThrough は
// format negotiation の対象外で常に PNG / 元 MIME を返す。
//
// video/* MIME はそのまま resize 経路に乗せず、まず still frame を ffmpeg で
// 抽出してから image pipeline に渡す。ffmpeg build tag 無しでは
// extractVideoThumbnailFrame が ErrVideoThumbnailUnavailable を返すので
// dummy PNG にフォールバックする (#637 M2)。
func (s *Service) processAndReturn(data []byte, contentType string, mode ProxyMode, out OutputFormat) (*ProxyResult, error) {
	if IsVideoMIME(contentType) && isResizeMode(mode) {
		frame, err := extractVideoThumbnailFrame(data, contentType)
		if err != nil {
			return makeDummyPNG(), nil
		}
		// still frame は image/jpeg として image pipeline に渡す。
		data = frame
		contentType = "image/jpeg"
	}
	switch mode {
	case ModeEmoji:
		return s.processResize(data, contentType, 0, emojiHeight, out)
	case ModeAvatar:
		return s.processResize(data, contentType, 0, avatarHeight, out)
	case ModeStatic:
		return s.processResize(data, contentType, staticWidth, staticHeight, out)
	case ModePreview:
		return s.processResize(data, contentType, previewWidth, previewHeight, out)
	case ModeBadge:
		return s.processBadge(data, contentType)
	default:
		return s.passThrough(data, contentType)
	}
}

// isResizeMode reports whether the mode triggers image-pipeline processing
// (so a video source needs a still frame extracted first).
func isResizeMode(mode ProxyMode) bool {
	switch mode {
	case ModeEmoji, ModeAvatar, ModeStatic, ModePreview, ModeBadge:
		return true
	}
	return false
}

// processResize decodes the image, resizes it, and encodes to WebP or AVIF.
// width=0の場合はheightのみでアスペクト比を維持する。out=FormatAVIF の
// ときは AVIF で書き出し、エンコード失敗時は WebP に fallback する。
func (s *Service) processResize(data []byte, contentType string, width, height int, out OutputFormat) (*ProxyResult, error) {
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

	if out == FormatAVIF {
		if encoded, err := encodeAVIF(resized); err == nil {
			return makeResult(encoded, "image/avif"), nil
		}
		// AVIF 失敗時は WebP に fallback (元データに戻すと sensitive な
		// resize 結果が伝わらないため)
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
		// IANA 公式名 (image/vnd.microsoft.icon) と古い慣例 (image/x-icon)
		// を両方許可する (#418)。
		"image/x-icon", "image/vnd.microsoft.icon",
		"image/vnd.mozilla.apng",
		// gen2brain wazero ベースの decoder で対応 (#637 M3/M4/M5):
		// image/avif (in/out), image/heic, image/heif, image/jxl は input 専用
		// として decode → WebP/AVIF 出力経路に乗せる。
		"image/avif", "image/heic", "image/heif", "image/jxl":
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
	// gen2brain/webp は内部で image.Image を任意の型から libwebp に渡せるが、
	// 互換性のため明示的に NRGBA に正規化しておく (chai2010 時代と同等の前段)。
	bounds := img.Bounds()
	nrgba := image.NewNRGBA(bounds)
	draw.Draw(nrgba, bounds, img, bounds.Min, draw.Src)

	var buf bytes.Buffer
	if err := webp.Encode(&buf, nrgba, webp.Options{Quality: webpQuality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// encodeAVIF encodes img as AVIF using gen2brain/avif (wazero based, no cgo).
// quality / speed は WebP より重いので avifSpeed=8 を default にしておく。
func encodeAVIF(img image.Image) ([]byte, error) {
	bounds := img.Bounds()
	nrgba := image.NewNRGBA(bounds)
	draw.Draw(nrgba, bounds, img, bounds.Min, draw.Src)

	var buf bytes.Buffer
	if err := avif.Encode(&buf, nrgba, avif.Options{Quality: avifQuality, Speed: avifSpeed}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
