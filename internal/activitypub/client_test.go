package activitypub

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_PostSigned(t *testing.T) {
	key, pub := newTestKey(t)

	var seenSig string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenSig = r.Header.Get("Signature")
		r.Header.Set("Host", r.Host)
		if err := VerifyRequest(r, pub); err != nil {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(nil, "misskey-go-test")
	resp, err := c.PostSigned(srv.URL+"/inbox", []byte(`{"type":"Create"}`), key)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotEmpty(t, seenSig)
}

func TestClient_PostSigned_BadURL(t *testing.T) {
	key, _ := newTestKey(t)
	c := NewClient(nil, "")
	_, err := c.PostSigned("://bad", []byte("x"), key)
	assert.Error(t, err)
}

func TestClient_PostSigned_SignError(t *testing.T) {
	c := NewClient(nil, "")
	_, err := c.PostSigned("https://x.example/", []byte("x"), nil)
	assert.Error(t, err)
}

func TestClient_GetSigned(t *testing.T) {
	key, pub := newTestKey(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("Host", r.Host)
		if err := VerifyRequest(r, pub); err != nil {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/activity+json")
		_, _ = w.Write([]byte(`{"id":"https://example.com/users/u1"}`))
	}))
	defer srv.Close()

	c := NewClient(nil, "")
	resp, err := c.GetSigned(srv.URL+"/users/u1", key, "")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "u1")
}

func TestClient_GetSigned_WithUserAgent(t *testing.T) {
	key, _ := newTestKey(t)
	var seenUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(nil, "misskey-go-test/1.0")
	resp, err := c.GetSigned(srv.URL+"/", key, "")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, "misskey-go-test/1.0", seenUA)
}

func TestClient_GetSigned_AcceptOverride(t *testing.T) {
	key, _ := newTestKey(t)
	var seenAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAccept = r.Header.Get("Accept")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(nil, "")
	resp, err := c.GetSigned(srv.URL+"/", key, "application/json")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, "application/json", seenAccept)
}

func TestClient_GetSigned_BadURL(t *testing.T) {
	key, _ := newTestKey(t)
	c := NewClient(nil, "")
	_, err := c.GetSigned("://bad", key, "")
	assert.Error(t, err)
}

func TestClient_GetSigned_SignError(t *testing.T) {
	c := NewClient(nil, "")
	_, err := c.GetSigned("https://x.example/", nil, "")
	assert.Error(t, err)
}

func TestClient_FetchJSON(t *testing.T) {
	key, _ := newTestKey(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := NewClient(nil, "")
	got, err := c.FetchJSON(srv.URL+"/x", key)
	require.NoError(t, err)
	assert.Contains(t, string(got), "ok")
}

func TestClient_FetchJSON_NonOK(t *testing.T) {
	key, _ := newTestKey(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(nil, "")
	_, err := c.FetchJSON(srv.URL, key)
	assert.Error(t, err)
}

func TestClient_FetchJSON_Error(t *testing.T) {
	key, _ := newTestKey(t)
	c := NewClient(nil, "")
	_, err := c.FetchJSON("://bad", key)
	assert.Error(t, err)
}

func TestNewClient_DefaultHTTPClient(t *testing.T) {
	c := NewClient(nil, "")
	assert.NotNil(t, c)
}
