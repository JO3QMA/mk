package federation

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func postStub(handler func(echo.Context) error) *httptest.ResponseRecorder {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	_ = handler(c)
	return rec
}

func TestFollowers(t *testing.T) {
	h, _ := newHandler(t)
	assert.Equal(t, http.StatusOK, postStub(h.Followers).Code)
}
func TestFollowing(t *testing.T) {
	h, _ := newHandler(t)
	assert.Equal(t, http.StatusOK, postStub(h.Following).Code)
}
func TestUsers(t *testing.T) {
	h, _ := newHandler(t)
	assert.Equal(t, http.StatusOK, postStub(h.Users).Code)
}
func TestStats(t *testing.T) {
	h, _ := newHandler(t)
	assert.Equal(t, http.StatusOK, postStub(h.Stats).Code)
}
func TestUpdateRemoteUser(t *testing.T) {
	h, _ := newHandler(t)
	assert.Equal(t, http.StatusNoContent, postStub(h.UpdateRemoteUser).Code)
}
