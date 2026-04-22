package translate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeepL_Translate_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DeepL-Auth-Key testkey", r.Header.Get("Authorization"))
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		_ = r.ParseForm()
		assert.Equal(t, "EN", r.FormValue("target_lang"))

		json.NewEncoder(w).Encode(deeplResponse{
			Translations: []struct {
				DetectedSourceLanguage string `json:"detected_source_language"`
				Text                   string `json:"text"`
			}{
				{DetectedSourceLanguage: "JA", Text: "Hello"},
			},
		})
	}))
	defer srv.Close()

	c := NewDeepLWithClient("testkey", srv.URL, srv.Client())
	result, err := c.Translate(context.Background(), "こんにちは", "en")
	require.NoError(t, err)
	assert.Equal(t, "ja", result.SourceLang)
	assert.Equal(t, "Hello", result.Text)
}

func TestDeepL_Translate_NoResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(deeplResponse{Translations: nil})
	}))
	defer srv.Close()

	c := NewDeepLWithClient("key", srv.URL, srv.Client())
	_, err := c.Translate(context.Background(), "text", "en")
	assert.ErrorIs(t, err, ErrNoResult)
}

func TestDeepL_Translate_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("Forbidden"))
	}))
	defer srv.Close()

	c := NewDeepLWithClient("badkey", srv.URL, srv.Client())
	_, err := c.Translate(context.Background(), "text", "en")
	assert.ErrorIs(t, err, ErrRequestFailed)
}

func TestDeepL_Translate_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	c := NewDeepLWithClient("key", srv.URL, srv.Client())
	_, err := c.Translate(context.Background(), "text", "en")
	assert.ErrorIs(t, err, ErrRequestFailed)
}

func TestDeepL_Translate_NetworkError(t *testing.T) {
	c := NewDeepLWithClient("key", "http://127.0.0.1:1", http.DefaultClient)
	_, err := c.Translate(context.Background(), "text", "en")
	assert.ErrorIs(t, err, ErrRequestFailed)
}

func TestNewDeepL_ProEndpoint(t *testing.T) {
	c := NewDeepL("key", true)
	assert.Equal(t, deeplProURL, c.apiURL)
}

func TestNewDeepL_FreeEndpoint(t *testing.T) {
	c := NewDeepL("key", false)
	assert.Equal(t, deeplFreeURL, c.apiURL)
}

// #340: DeepL fetcher が過大 response を safehttp.ReadAllLimit で弾くこと。
// 1 MiB cap を超える body を返す stub server に対して ErrRequestFailed を返す。
func TestDeepL_Translate_ResponseTooLarge(t *testing.T) {
	oversized := make([]byte, 2<<20)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write(oversized)
	}))
	defer srv.Close()

	c := NewDeepLWithClient("key", srv.URL, srv.Client())
	_, err := c.Translate(context.Background(), "text", "en")
	assert.ErrorIs(t, err, ErrRequestFailed)
}
