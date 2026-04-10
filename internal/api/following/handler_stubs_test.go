package following

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func postStub(handler func(echo.Context) error, body string) *httptest.ResponseRecorder {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	_ = handler(c)
	return rec
}

func TestInvalidate(t *testing.T) {
	h, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, postStub(h.Invalidate, `{}`).Code)
}

func TestUpdateFollow(t *testing.T) {
	h, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, postStub(h.UpdateFollow, `{}`).Code)
}

func TestUpdateFollowAll(t *testing.T) {
	h, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, postStub(h.UpdateFollowAll, `{}`).Code)
}

func TestRequestsSent(t *testing.T) {
	h, _ := newTestHandler(t)
	assert.Equal(t, http.StatusOK, postStub(h.RequestsSent, `{}`).Code)
}
