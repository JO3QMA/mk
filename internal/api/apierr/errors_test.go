package apierr

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestError_Structure(t *testing.T) {
	result := Error("INVALID_PARAM", "Invalid.", "abc-123")
	errObj, ok := result["error"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "Invalid.", errObj["message"])
	assert.Equal(t, "INVALID_PARAM", errObj["code"])
	assert.Equal(t, "abc-123", errObj["id"])
}

func TestInvalidParam(t *testing.T) {
	result := InvalidParam()
	errObj := result["error"].(map[string]any)
	assert.Equal(t, "INVALID_PARAM", errObj["code"])
	assert.Equal(t, UUIDInvalidParam, errObj["id"])
}

func TestInternalError(t *testing.T) {
	result := InternalError()
	errObj := result["error"].(map[string]any)
	assert.Equal(t, "INTERNAL_ERROR", errObj["code"])
	assert.Equal(t, UUIDInternalError, errObj["id"])
}

func TestNoSuchNote(t *testing.T) {
	result := NoSuchNote()
	errObj := result["error"].(map[string]any)
	assert.Equal(t, "NO_SUCH_NOTE", errObj["code"])
	assert.Equal(t, UUIDNoSuchNote, errObj["id"])
}

func TestNoSuchUser(t *testing.T) {
	result := NoSuchUser()
	errObj := result["error"].(map[string]any)
	assert.Equal(t, "NO_SUCH_USER", errObj["code"])
	assert.Equal(t, UUIDNoSuchUser, errObj["id"])
}

func TestAccessDenied(t *testing.T) {
	result := AccessDenied()
	errObj := result["error"].(map[string]any)
	assert.Equal(t, "ACCESS_DENIED", errObj["code"])
	assert.Equal(t, UUIDAccessDenied, errObj["id"])
}
