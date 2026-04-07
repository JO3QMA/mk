package federation

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shiroha-a/mk/internal/activitypub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPFetcher_FetchActor(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/activity+json")
		_, _ = w.Write([]byte(`{"id":"x"}`))
	}))
	defer srv.Close()

	c := activitypub.NewClient(nil, "test")
	f := NewAPFetcher(c)
	body, err := f.FetchActor(srv.URL + "/users/alice")
	require.NoError(t, err)
	assert.Contains(t, string(body), "x")
}
