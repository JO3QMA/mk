package federation

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shiroha-a/mk/internal/activitypub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPFetcher_FetchObject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/activity+json")
		_, _ = w.Write([]byte(`{"id":"x"}`))
	}))
	defer srv.Close()

	c := activitypub.NewClient(nil, "test")
	f := NewAPFetcher(c)
	body, err := f.FetchObject(srv.URL + "/users/alice")
	require.NoError(t, err)
	assert.Contains(t, string(body), "x")
}

func TestAPFetcher_FetchHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><link rel="icon" href="/a.png"></head></html>`))
	}))
	defer srv.Close()

	c := activitypub.NewClient(nil, "test")
	f := NewAPFetcher(c)
	body, err := f.FetchHTML(srv.URL)
	require.NoError(t, err)
	assert.Contains(t, string(body), "<link rel=\"icon\"")
}
