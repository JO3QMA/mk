package mediaproxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrVideoThumbnailUnavailable is returned by fetchVideoThumbnail when no
// videoThumbnailGenerator is configured or the upstream call failed.
var ErrVideoThumbnailUnavailable = errors.New("mediaproxy: video thumbnail unavailable")

// videoThumbnailTimeout caps the round-trip to the external thumbnail
// generator. Generators normally finish in well under a second; longer
// than this we suspect a stalled decoder or unreachable service.
const videoThumbnailTimeout = 30 * time.Second

// isVideoMIME reports whether contentType is a video/* MIME type the proxy
// should attempt to extract a still frame from.
func isVideoMIME(contentType string) bool {
	return strings.HasPrefix(strings.ToLower(contentType), "video/")
}

// newVideoThumbnailClient returns an *http.Client wired to talk to genURL.
//
//   - `unix:///path/to/socket`  → HTTP over a Unix domain socket. The URL
//     authority (host) is ignored and replaced with a constant placeholder
//     when building requests; the dialer connects to the socket path.
//   - everything else            → ordinary HTTP/HTTPS over TCP.
//
// The generator is operator-configured and assumed trusted, so SSRF guard
// is intentionally not applied (mirroring urlpreview's proxyClient pattern,
// #638). Returns nil when genURL is empty (feature disabled).
func newVideoThumbnailClient(genURL string) *http.Client {
	if genURL == "" {
		return nil
	}
	if strings.HasPrefix(genURL, "unix:") {
		u, err := url.Parse(genURL)
		if err != nil || u.Path == "" {
			return nil
		}
		socketPath := u.Path
		return &http.Client{
			Timeout: videoThumbnailTimeout,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
				},
			},
		}
	}
	return &http.Client{Timeout: videoThumbnailTimeout}
}

// videoThumbnailRequestURL builds the request URL the generator expects.
//
// mode == "post" (default, nekonoverse/video-thumb): `POST <base>/thumbnail`
// with `multipart/form-data` (`file` field). sourceURL は使われない (bytes
// で送る)。
//
// mode == "get" (Misskey TS-compatible): `GET <base>/thumbnail.webp?
// thumbnail=1&url=<encoded>`. service 側で URL を fetch する前提。
//
// For UDS the original URL's authority and path are the socket path itself
// (not part of the HTTP request line), so we drop them entirely and use
// `http://localhost` as the placeholder host. `localhost` is what the
// upstream service is most likely to accept in its Host header.
func videoThumbnailRequestURL(genURL, mode, sourceURL string) (string, error) {
	base := ""
	if strings.HasPrefix(genURL, "unix:") {
		// `url.Parse` is only used to validate the input; the resulting
		// fields are intentionally ignored — see comment above.
		if _, err := url.Parse(genURL); err != nil {
			return "", err
		}
		base = "http://localhost"
	} else {
		base = strings.TrimRight(genURL, "/")
	}
	if mode == "get" {
		return base + "/thumbnail.webp?thumbnail=1&url=" + url.QueryEscape(sourceURL), nil
	}
	return base + "/thumbnail", nil
}

// fetchVideoThumbnail dispatches to the POST or GET wire based on the
// configured mode and returns the still frame thumbnail (typically WebP).
// The caller pipes the result back through processAndReturn for resize /
// format negotiation.
func (s *Service) fetchVideoThumbnail(ctx context.Context, body []byte, sourceMIME, sourceURL string) ([]byte, string, error) {
	if s.videoThumbClient == nil {
		return nil, "", ErrVideoThumbnailUnavailable
	}
	if s.videoThumbMode == "get" {
		return s.fetchVideoThumbnailGET(ctx, sourceURL)
	}
	return s.fetchVideoThumbnailPOST(ctx, body, sourceMIME)
}

// fetchVideoThumbnailPOST uploads the video bytes via multipart POST to
// nekonoverse/video-thumb-style endpoints. Bytes are forwarded from the
// already-downloaded `body`, so the generator does not need to reach the
// source URL itself (no extra SSRF surface on the generator).
//
// multipart body は io.Pipe で streaming する: 32 MiB 級の bytes をもう一
// 度 buffer に full コピーしないことで peak memory を半分にする (review
// PR #646 #2)。
func (s *Service) fetchVideoThumbnailPOST(ctx context.Context, body []byte, sourceMIME string) ([]byte, string, error) {
	target, err := videoThumbnailRequestURL(s.videoThumbGen, "post", "")
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrVideoThumbnailUnavailable, err)
	}

	pr, pw := io.Pipe()
	w := multipart.NewWriter(pw)
	// field name "file" は nekonoverse/video-thumb の FastAPI endpoint が
	// 期待する固定値。filename はリモート URL から取らない (個人情報を
	// generator に伝えない方針) — 拡張子だけ MIME から推定して generic
	// に。
	filename := "video" + extensionForMIME(sourceMIME)

	// goroutine で part の書き込みを行い、req.Body の読み手と pipe で
	// 接続する。エラーは pw.CloseWithError で reader 側に伝播させて、
	// http.Client.Do() が正しく失敗を観測できるようにする。
	go func() {
		defer pw.Close()
		part, err := w.CreateFormFile("file", filename)
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		if _, err := part.Write(body); err != nil {
			pw.CloseWithError(err)
			return
		}
		if err := w.Close(); err != nil {
			pw.CloseWithError(err)
			return
		}
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, pr)
	if err != nil {
		_ = pr.Close()
		return nil, "", fmt.Errorf("%w: %v", ErrVideoThumbnailUnavailable, err)
	}
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return s.doVideoThumbnailRequest(req)
}

// fetchVideoThumbnailGET asks a Misskey TS-compatible generator to fetch
// sourceURL itself. Unlike POST mode, the generator must be able to reach
// sourceURL from its network namespace.
func (s *Service) fetchVideoThumbnailGET(ctx context.Context, sourceURL string) ([]byte, string, error) {
	target, err := videoThumbnailRequestURL(s.videoThumbGen, "get", sourceURL)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrVideoThumbnailUnavailable, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrVideoThumbnailUnavailable, err)
	}
	req.Header.Set("User-Agent", s.userAgent)
	return s.doVideoThumbnailRequest(req)
}

// doVideoThumbnailRequest executes the prepared request and validates the
// response shape (status / size cap / content type). Shared between POST
// and GET wires.
func (s *Service) doVideoThumbnailRequest(req *http.Request) ([]byte, string, error) {
	resp, err := s.videoThumbClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrVideoThumbnailUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("%w: status %d", ErrVideoThumbnailUnavailable, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxDownload+1))
	if err != nil {
		return nil, "", fmt.Errorf("%w: read body: %v", ErrVideoThumbnailUnavailable, err)
	}
	if int64(len(data)) > maxDownload {
		// Wrap so callers can match a single sentinel for any
		// generator-side failure; the underlying ErrTooLarge stays in the
		// chain via errors.Is for callers that care.
		return nil, "", fmt.Errorf("%w: %w", ErrVideoThumbnailUnavailable, ErrTooLarge)
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	// Generator が malicious / 設定ミスで非画像 (HTML / JSON / text) を
	// 返した場合、processResize は non-convertible として raw bytes を
	// そのまま frontend に流してしまう。image/* で始まらない response は
	// 拒否して dummy PNG fallback に倒れる (review PR #646 #3)。
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		return nil, "", fmt.Errorf("%w: unexpected content-type %q", ErrVideoThumbnailUnavailable, contentType)
	}
	return data, contentType, nil
}

// extensionForMIME maps a small set of common video MIME types to filename
// extensions. Unknown / arbitrary MIMEs fall back to ".bin" — the generator
// uses ffmpeg autodetect which doesn't care about filename, so this only
// helps debugging.
func extensionForMIME(mime string) string {
	switch strings.ToLower(mime) {
	case "video/mp4", "video/x-m4v":
		return ".mp4"
	case "video/webm":
		return ".webm"
	case "video/quicktime":
		return ".mov"
	case "video/3gpp":
		return ".3gp"
	case "video/3gpp2":
		return ".3g2"
	case "video/mpeg":
		return ".mpg"
	case "video/ogg":
		return ".ogv"
	case "video/x-matroska":
		return ".mkv"
	case "video/mp2t":
		return ".ts"
	}
	return ".bin"
}
