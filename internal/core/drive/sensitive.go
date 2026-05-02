package drive

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// SensitiveDetector scores media content for NSFW likelihood.
// Implementations may call external services (e.g., NudeNet).
type SensitiveDetector interface {
	// Detect returns a score in [0, 1] where 1 = definitely NSFW.
	Detect(ctx context.Context, body []byte, mime string) (float64, error)
}

// SensitiveConfig holds meta-driven sensitive detection settings.
type SensitiveConfig struct {
	// Detection: "none", "all", "local", "remote"
	Detection string
	// Sensitivity: "veryLow", "low", "medium", "high", "veryHigh"
	Sensitivity string
	// SetFlagAutomatically updates DriveFile.IsSensitive based on detection.
	SetFlagAutomatically bool
	// EnableForVideos enables detection for video MIME types.
	EnableForVideos bool
	// SilencedHosts forces IsSensitive=true for media from these hosts.
	SilencedHosts []string
}

// SensitivityThreshold maps sensitivity names to score thresholds.
// score >= threshold → sensitive.
func SensitivityThreshold(sensitivity string) float64 {
	switch strings.ToLower(sensitivity) {
	case "verylow":
		return 0.95
	case "low":
		return 0.8
	case "medium":
		return 0.5
	case "high":
		return 0.3
	case "veryhigh":
		return 0.1
	default:
		return 0.5
	}
}

// IsSilencedHost reports whether host is in the silenced hosts list
// (case-insensitive, subdomain matching like bannedEmailDomains).
func IsSilencedHost(host string, silencedHosts []string) bool {
	if host == "" {
		return false
	}
	suffix := "." + strings.ToLower(host)
	for _, h := range silencedHosts {
		h = strings.TrimSpace(strings.ToLower(h))
		if h == "" {
			continue
		}
		if strings.HasSuffix(suffix, "."+h) {
			return true
		}
	}
	return false
}

// ShouldDetect returns whether detection should run for the given upload
// context based on the Detection mode.
func ShouldDetect(cfg SensitiveConfig, isLocalUser bool) bool {
	switch cfg.Detection {
	case "all":
		return true
	case "local":
		return isLocalUser
	case "remote":
		return !isLocalUser
	default:
		return false
	}
}

// IsVideoMIME reports whether the MIME type is a video type.
func IsVideoMIME(mime string) bool {
	return strings.HasPrefix(mime, "video/")
}

// HTTPDetector calls an external HTTP service for NSFW detection.
// リクエスト: POST <url> with body bytes, Content-Type header.
// レスポンス: JSON { "score": float64 }.
type HTTPDetector struct {
	url    string
	client *http.Client
}

// NewHTTPDetector creates a detector that calls the given URL.
//
// client は detector が POST に使う outbound HTTP client。nil なら 30s
// timeout の素の Client にフォールバックするが、production では SSRF-safe
// transport + forward proxy 経由の client を渡すこと (#638)。NSFW SaaS は
// operator が信頼する endpoint だが、operator 設定で outbound 経路を集約
// したい場合のため forward proxy には乗せる。
func NewHTTPDetector(url string, client *http.Client) *HTTPDetector {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &HTTPDetector{
		url:    url,
		client: client,
	}
}

func (d *HTTPDetector) Detect(ctx context.Context, body []byte, mime string) (float64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.url, io.NopCloser(strings.NewReader(string(body))))
	if err != nil {
		return 0, fmt.Errorf("sensitive: request creation: %w", err)
	}
	req.Header.Set("Content-Type", mime)

	resp, err := d.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("sensitive: request failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Score float64 `json:"score"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("sensitive: parse response: %w", err)
	}
	return result.Score, nil
}
