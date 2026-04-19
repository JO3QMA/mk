package activitypub

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestWebFingerClient(t *testing.T, handler http.HandlerFunc) (*WebFingerClient, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := NewWebFingerClient(srv.Client(), "test-agent")
	c.SetEndpointOverride(func(_ string) string { return srv.URL + "/.well-known/webfinger" })
	return c, srv
}

func TestWebFingerClient_LookupActorURI_Success(t *testing.T) {
	c, srv := newTestWebFingerClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/.well-known/webfinger", r.URL.Path)
		resource := r.URL.Query().Get("resource")
		assert.Equal(t, "acct:alice@example.com", resource)
		assert.Contains(t, r.Header.Get("Accept"), "application/jrd+json")
		assert.Equal(t, "test-agent", r.Header.Get("User-Agent"))
		w.Header().Set("Content-Type", "application/jrd+json")
		fmt.Fprint(w, `{
			"subject": "acct:alice@example.com",
			"links": [
				{"rel": "http://webfinger.net/rel/profile-page", "type": "text/html", "href": "https://example.com/@alice"},
				{"rel": "self", "type": "application/activity+json", "href": "https://example.com/users/alice"}
			]
		}`)
	})
	_ = srv
	uri, err := c.LookupActorURI("alice", "example.com")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/users/alice", uri)
}

func TestWebFingerClient_LookupActorURI_LDJSONTypeAccepted(t *testing.T) {
	c, _ := newTestWebFingerClient(t, func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{
			"subject": "acct:alice@example.com",
			"links": [
				{"rel": "self", "type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\"", "href": "https://example.com/users/alice"}
			]
		}`)
	})
	uri, err := c.LookupActorURI("alice", "example.com")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/users/alice", uri)
}

func TestWebFingerClient_LookupActorURI_NoSelfLink(t *testing.T) {
	c, _ := newTestWebFingerClient(t, func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{
			"subject": "acct:alice@example.com",
			"links": [
				{"rel": "http://webfinger.net/rel/profile-page", "type": "text/html", "href": "https://example.com/@alice"}
			]
		}`)
	})
	_, err := c.LookupActorURI("alice", "example.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no self link")
}

func TestWebFingerClient_LookupActorURI_SelfLinkWithUnrelatedType(t *testing.T) {
	c, _ := newTestWebFingerClient(t, func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{
			"subject": "acct:alice@example.com",
			"links": [
				{"rel": "self", "type": "text/html", "href": "https://example.com/@alice"}
			]
		}`)
	})
	_, err := c.LookupActorURI("alice", "example.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no self link")
}

func TestWebFingerClient_LookupActorURI_SelfLinkEmptyHref(t *testing.T) {
	c, _ := newTestWebFingerClient(t, func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{
			"subject": "acct:alice@example.com",
			"links": [
				{"rel": "self", "type": "application/activity+json"}
			]
		}`)
	})
	_, err := c.LookupActorURI("alice", "example.com")
	require.Error(t, err)
}

func TestWebFingerClient_LookupActorURI_InvalidJSON(t *testing.T) {
	c, _ := newTestWebFingerClient(t, func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `not json`)
	})
	_, err := c.LookupActorURI("alice", "example.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse response")
}

func TestWebFingerClient_LookupActorURI_Non2xx(t *testing.T) {
	c, _ := newTestWebFingerClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	_, err := c.LookupActorURI("ghost", "example.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 404")
}

func TestWebFingerClient_LookupActorURI_NetworkError(t *testing.T) {
	c := NewWebFingerClient(http.DefaultClient, "ua")
	c.SetEndpointOverride(func(_ string) string { return "http://127.0.0.1:1/.well-known/webfinger" })
	_, err := c.LookupActorURI("a", "example.com")
	require.Error(t, err)
}

func TestWebFingerClient_LookupActorURI_EmptyArgs(t *testing.T) {
	c := NewWebFingerClient(nil, "ua")
	_, err := c.LookupActorURI("", "example.com")
	require.Error(t, err)
	_, err = c.LookupActorURI("alice", "")
	require.Error(t, err)
}

func TestWebFingerClient_DefaultEndpoint(t *testing.T) {
	// SetEndpointOverride を使わないデフォルト経路の確認。
	// 実ネットワークには行かないが、URL が https://<host>/.well-known/webfinger で
	// 組まれていることを確認する。
	c := NewWebFingerClient(nil, "ua")
	got := c.endpoint("example.com")
	assert.Equal(t, "https://example.com/.well-known/webfinger", got)
	assert.True(t, strings.HasPrefix(got, "https://"))
}

func TestWebFingerClient_ResourceIsURLEncoded(t *testing.T) {
	c, _ := newTestWebFingerClient(t, func(w http.ResponseWriter, r *http.Request) {
		raw := r.URL.RawQuery
		// query parser decodes the value for us, but the raw must contain
		// a percent-encoded "@" as part of a correct resource query.
		decoded, _ := url.ParseQuery(raw)
		assert.Equal(t, "acct:edge@ex.example", decoded.Get("resource"))
		fmt.Fprint(w, `{
			"subject": "acct:edge@ex.example",
			"links": [{"rel": "self", "type": "application/activity+json", "href": "https://ex.example/users/edge"}]
		}`)
	})
	uri, err := c.LookupActorURI("edge", "ex.example")
	require.NoError(t, err)
	assert.Equal(t, "https://ex.example/users/edge", uri)
}

func TestIsActivityPubLinkType(t *testing.T) {
	cases := map[string]bool{
		"application/activity+json": true,
		"application/ld+json":       true,
		"application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\"": true,
		"application/jrd+json": false,
		"text/html":            false,
		"":                     false,
	}
	for input, want := range cases {
		assert.Equal(t, want, isActivityPubLinkType(input), input)
	}
}
