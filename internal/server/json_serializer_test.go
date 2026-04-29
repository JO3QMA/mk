package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withSerializer prepares an echo.Context with the fastJSONSerializer
// installed, mimicking what server.New does at boot. Currently backed by
// stdlib encoding/json (#542 で goccy 0.10.x の VM compile bug が
// timeline 経路で踏まれたため一時的に巻き戻し)。
func withSerializer(body string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	e.JSONSerializer = fastJSONSerializer{}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	return c, rec
}

func TestFastJSONSerializer_SerializeMatchesStdlib(t *testing.T) {
	c, rec := withSerializer("")
	payload := map[string]any{"name": "alice", "n": 42, "ok": true}

	require.NoError(t, c.JSON(http.StatusOK, payload))

	assert.Equal(t, http.StatusOK, rec.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "alice", got["name"])
	assert.Equal(t, float64(42), got["n"])
	assert.Equal(t, true, got["ok"])
}

func TestFastJSONSerializer_SerializeIndent(t *testing.T) {
	c, rec := withSerializer("")
	require.NoError(t, c.JSONPretty(http.StatusOK, map[string]any{"x": 1}, "  "))
	assert.Contains(t, rec.Body.String(), "\n", "indent should produce newlines")
}

func TestFastJSONSerializer_DeserializeOK(t *testing.T) {
	c, _ := withSerializer(`{"name":"bob","n":7}`)
	var dst struct {
		Name string `json:"name"`
		N    int    `json:"n"`
	}
	require.NoError(t, c.Bind(&dst))
	assert.Equal(t, "bob", dst.Name)
	assert.Equal(t, 7, dst.N)
}

// 不正な JSON は echo.HTTPError(400) に翻訳されること (stdlib 実装と同じ)。
func TestFastJSONSerializer_DeserializeSyntaxError(t *testing.T) {
	c, _ := withSerializer(`{"name": "bob"`) // closing brace 無し
	var dst struct {
		Name string `json:"name"`
	}
	err := c.Bind(&dst)
	require.Error(t, err)
	httpErr, ok := err.(*echo.HTTPError)
	require.True(t, ok, "want *echo.HTTPError, got %T", err)
	assert.Equal(t, http.StatusBadRequest, httpErr.Code)
}

// 型不一致 (string 期待のフィールドに number) も同様に 400 になること。
func TestFastJSONSerializer_DeserializeTypeError(t *testing.T) {
	c, _ := withSerializer(`{"name": 42}`)
	var dst struct {
		Name string `json:"name"`
	}
	err := c.Bind(&dst)
	require.Error(t, err)
	httpErr, ok := err.(*echo.HTTPError)
	require.True(t, ok, "want *echo.HTTPError, got %T", err)
	assert.Equal(t, http.StatusBadRequest, httpErr.Code)
}
