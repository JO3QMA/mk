package channels

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

func TestFavorite(t *testing.T) {
	h, _, _, _ := newHandler(t)
	assert.Equal(t, http.StatusNoContent, postStub(h.Favorite).Code)
}
func TestUnfavorite(t *testing.T) {
	h, _, _, _ := newHandler(t)
	assert.Equal(t, http.StatusNoContent, postStub(h.Unfavorite).Code)
}
func TestMuteCreate(t *testing.T) {
	h, _, _, _ := newHandler(t)
	assert.Equal(t, http.StatusNoContent, postStub(h.MuteCreate).Code)
}
func TestMuteDelete(t *testing.T) {
	h, _, _, _ := newHandler(t)
	assert.Equal(t, http.StatusNoContent, postStub(h.MuteDelete).Code)
}
func TestMyFavorites(t *testing.T) {
	h, _, _, _ := newHandler(t)
	assert.Equal(t, http.StatusOK, postStub(h.MyFavorites).Code)
}
func TestMuteList(t *testing.T) {
	h, _, _, _ := newHandler(t)
	assert.Equal(t, http.StatusOK, postStub(h.MuteList).Code)
}
