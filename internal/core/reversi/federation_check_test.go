package reversi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- stubHTTPDoer ---

type stubHTTPDoer struct {
	responses map[string]string
	calls     int
}

func (s *stubHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	s.calls++
	body, ok := s.responses[req.URL.String()]
	if !ok {
		return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(""))}, nil
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func newStubDoer(reversiVersion string) *stubHTTPDoer {
	discovery, _ := json.Marshal(map[string]any{
		"links": []map[string]any{
			{"rel": "http://nodeinfo.diaspora.software/ns/schema/2.1", "href": "https://remote.example/nodeinfo/2.1"},
		},
	})
	nodeinfo, _ := json.Marshal(map[string]any{
		"metadata": map[string]any{"reversiVersion": reversiVersion},
	})
	return &stubHTTPDoer{responses: map[string]string{
		"https://remote.example/.well-known/nodeinfo": string(discovery),
		"https://remote.example/nodeinfo/2.1":         string(nodeinfo),
	}}
}

// --- tests ---

func TestFederationChecker_AvailableMajorMatch(t *testing.T) {
	doer := newStubDoer("1.2.5-yojo")
	c := NewFederationChecker(nil, doer)
	assert.True(t, c.Available(context.Background(), "remote.example"))
}

func TestFederationChecker_AvailableMajorMismatch(t *testing.T) {
	doer := newStubDoer("2.0.0-foo")
	c := NewFederationChecker(nil, doer)
	assert.False(t, c.Available(context.Background(), "remote.example"))
}

func TestFederationChecker_EmptyReversiVersion(t *testing.T) {
	doer := newStubDoer("")
	c := NewFederationChecker(nil, doer)
	assert.False(t, c.Available(context.Background(), "remote.example"))
}

func TestFederationChecker_DiscoveryFailure(t *testing.T) {
	// unknown host → 404 → empty version → false
	c := NewFederationChecker(nil, &stubHTTPDoer{responses: map[string]string{}})
	assert.False(t, c.Available(context.Background(), "unknown.example"))
}

func TestFederationChecker_NilChecker(t *testing.T) {
	var c *FederationChecker
	assert.False(t, c.Available(context.Background(), "remote.example"))
}

func TestFederationChecker_EmptyHost(t *testing.T) {
	c := NewFederationChecker(nil, &stubHTTPDoer{})
	assert.False(t, c.Available(context.Background(), ""))
}

func TestMajorCompatible(t *testing.T) {
	// ReversiVersion は "1.1.0-mkgo"
	require.True(t, strings.HasPrefix(ReversiVersion, "1."))
	assert.True(t, majorCompatible("1.1.0-mkgo"))
	assert.True(t, majorCompatible("1.2.5-yojo"))
	assert.False(t, majorCompatible("2.0.0-foo"))
	assert.False(t, majorCompatible(""))
}

func TestFirstDotSegment(t *testing.T) {
	assert.Equal(t, "1", firstDotSegment("1.1.0-mkgo"))
	assert.Equal(t, "2", firstDotSegment("2.0"))
	assert.Equal(t, "1", firstDotSegment("1-foo"))
	assert.Equal(t, "1", firstDotSegment("1"))
}
