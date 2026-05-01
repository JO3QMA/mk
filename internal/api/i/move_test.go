package i

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMoveAccount(t *testing.T) {
	h, _ := newExtraHandler(t)
	// moveToAccount / password 未指定は 400
	assert.Equal(t, http.StatusBadRequest, postExtra(h.Move, `{}`, stubUser).Code)
	assert.Equal(t, http.StatusBadRequest, postExtra(h.Move, `{"moveToAccount":"https://x"}`, stubUser).Code)
	// profile 未登録 → ACCESS_DENIED
	assert.Equal(t, http.StatusForbidden, postExtra(h.Move, `{"moveToAccount":"https://x","password":"pw"}`, stubUser).Code)
}
