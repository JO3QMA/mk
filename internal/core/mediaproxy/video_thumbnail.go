package mediaproxy

import (
	"context"
	"errors"
	"fmt"
	"io"
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

// videoThumbnailRequestURL builds the GET URL the generator expects.
// Misskey TS' videoThumbnailGenerator API: `<base>/thumbnail.webp?thumbnail=1&url=<encoded>`.
//
// For UDS the original URL's authority and path are the socket path itself
// (not part of the HTTP request line), so we drop them entirely and use
// `http://localhost` as the placeholder host. `localhost` is what the
// upstream service is most likely to accept in its Host header.
func videoThumbnailRequestURL(genURL, sourceURL string) (string, error) {
	if strings.HasPrefix(genURL, "unix:") {
		// `url.Parse` is only used to validate the input; the resulting
		// fields are intentionally ignored — see comment above.
		if _, err := url.Parse(genURL); err != nil {
			return "", err
		}
		return "http://localhost/thumbnail.webp?thumbnail=1&url=" + url.QueryEscape(sourceURL), nil
	}
	return strings.TrimRight(genURL, "/") + "/thumbnail.webp?thumbnail=1&url=" + url.QueryEscape(sourceURL), nil
}

// fetchVideoThumbnail asks the configured external generator for a still
// frame thumbnail of sourceURL. The returned bytes are an image (typically
// WebP) that the caller will pipe through processAndReturn for further
// resizing / format negotiation.
func (s *Service) fetchVideoThumbnail(ctx context.Context, sourceURL string) ([]byte, string, error) {
	if s.videoThumbClient == nil {
		return nil, "", ErrVideoThumbnailUnavailable
	}
	target, err := videoThumbnailRequestURL(s.videoThumbGen, sourceURL)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrVideoThumbnailUnavailable, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrVideoThumbnailUnavailable, err)
	}
	req.Header.Set("User-Agent", s.userAgent)
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
	return data, contentType, nil
}
