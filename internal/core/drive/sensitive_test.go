package drive

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSensitivityThreshold(t *testing.T) {
	assert.Equal(t, 0.95, SensitivityThreshold("veryLow"))
	assert.Equal(t, 0.8, SensitivityThreshold("low"))
	assert.Equal(t, 0.5, SensitivityThreshold("medium"))
	assert.Equal(t, 0.3, SensitivityThreshold("high"))
	assert.Equal(t, 0.1, SensitivityThreshold("veryHigh"))
	assert.Equal(t, 0.5, SensitivityThreshold("unknown"))
}

func TestIsSilencedHost(t *testing.T) {
	hosts := []string{"bad.example.com", "spam.org"}

	assert.True(t, IsSilencedHost("bad.example.com", hosts))
	assert.True(t, IsSilencedHost("sub.bad.example.com", hosts))
	assert.True(t, IsSilencedHost("spam.org", hosts))
	assert.False(t, IsSilencedHost("good.example.com", hosts))
	assert.False(t, IsSilencedHost("", hosts))
	assert.False(t, IsSilencedHost("anything.com", nil))
}

func TestIsSilencedHost_EmptyEntries(t *testing.T) {
	assert.False(t, IsSilencedHost("anything.com", []string{"", "  "}))
}

func TestShouldDetect(t *testing.T) {
	assert.True(t, ShouldDetect(SensitiveConfig{Detection: "all"}, true))
	assert.True(t, ShouldDetect(SensitiveConfig{Detection: "all"}, false))
	assert.True(t, ShouldDetect(SensitiveConfig{Detection: "local"}, true))
	assert.False(t, ShouldDetect(SensitiveConfig{Detection: "local"}, false))
	assert.False(t, ShouldDetect(SensitiveConfig{Detection: "remote"}, true))
	assert.True(t, ShouldDetect(SensitiveConfig{Detection: "remote"}, false))
	assert.False(t, ShouldDetect(SensitiveConfig{Detection: "none"}, true))
	assert.False(t, ShouldDetect(SensitiveConfig{Detection: ""}, true))
}

func TestIsVideoMIME(t *testing.T) {
	assert.True(t, IsVideoMIME("video/mp4"))
	assert.True(t, IsVideoMIME("video/webm"))
	assert.False(t, IsVideoMIME("image/png"))
	assert.False(t, IsVideoMIME("audio/mp3"))
}

func TestHTTPDetector_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "image/png", r.Header.Get("Content-Type"))
		json.NewEncoder(w).Encode(map[string]any{"score": 0.85})
	}))
	defer srv.Close()

	d := NewHTTPDetector(srv.URL)
	score, err := d.Detect(context.Background(), []byte("fake-image"), "image/png")
	require.NoError(t, err)
	assert.InDelta(t, 0.85, score, 0.001)
}

func TestHTTPDetector_NetworkError(t *testing.T) {
	d := NewHTTPDetector("http://127.0.0.1:1")
	_, err := d.Detect(context.Background(), []byte("data"), "image/png")
	assert.Error(t, err)
}

func TestHTTPDetector_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	d := NewHTTPDetector(srv.URL)
	_, err := d.Detect(context.Background(), []byte("data"), "image/png")
	assert.Error(t, err)
}
