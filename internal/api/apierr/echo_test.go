package apierr

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func invoke(t *testing.T, fn func(echo.Context) error) (int, map[string]any) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	_ = fn(c)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return rec.Code, body
}

func TestJSONInvalidParam(t *testing.T) {
	code, body := invoke(t, JSONInvalidParam)
	assert.Equal(t, http.StatusBadRequest, code)
	errObj := body["error"].(map[string]any)
	assert.Equal(t, "INVALID_PARAM", errObj["code"])
	assert.Equal(t, UUIDInvalidParam, errObj["id"])
}

func TestJSONInternalError(t *testing.T) {
	code, body := invoke(t, JSONInternalError)
	assert.Equal(t, http.StatusInternalServerError, code)
	errObj := body["error"].(map[string]any)
	assert.Equal(t, "INTERNAL_ERROR", errObj["code"])
	assert.Equal(t, UUIDInternalError, errObj["id"])
}

func TestJSONNoSuchUser(t *testing.T) {
	code, body := invoke(t, JSONNoSuchUser)
	assert.Equal(t, http.StatusNotFound, code)
	errObj := body["error"].(map[string]any)
	assert.Equal(t, "NO_SUCH_USER", errObj["code"])
	assert.Equal(t, UUIDNoSuchUser, errObj["id"])
}

func TestJSONNoSuchNote(t *testing.T) {
	code, body := invoke(t, JSONNoSuchNote)
	assert.Equal(t, http.StatusNotFound, code)
	errObj := body["error"].(map[string]any)
	assert.Equal(t, "NO_SUCH_NOTE", errObj["code"])
	assert.Equal(t, UUIDNoSuchNote, errObj["id"])
}

func TestJSONAccessDenied(t *testing.T) {
	code, body := invoke(t, JSONAccessDenied)
	assert.Equal(t, http.StatusForbidden, code)
	errObj := body["error"].(map[string]any)
	assert.Equal(t, "ACCESS_DENIED", errObj["code"])
	assert.Equal(t, UUIDAccessDenied, errObj["id"])
}
