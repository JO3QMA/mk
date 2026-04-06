package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestRateLimiter_AllowsWithinLimit(t *testing.T) {
	rl := NewRateLimiter()
	e := echo.New()

	handler := rl.Limit(RateLimitConfig{
		Duration: time.Minute,
		Max:      3,
	})(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPath("/test")

		err := handler(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
	}
}

func TestRateLimiter_RejectsOverLimit(t *testing.T) {
	rl := NewRateLimiter()
	e := echo.New()

	handler := rl.Limit(RateLimitConfig{
		Duration: time.Minute,
		Max:      2,
	})(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	// 最初の2リクエストは通過
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPath("/test")

		err := handler(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
	}

	// 3番目は429
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/test")

	err := handler(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))
}

func TestRateLimiter_ResetsAfterDuration(t *testing.T) {
	rl := NewRateLimiter()
	e := echo.New()

	handler := rl.Limit(RateLimitConfig{
		Duration: 50 * time.Millisecond,
		Max:      1,
	})(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	// 最初のリクエスト: 成功
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/test")
	_ = handler(c)
	assert.Equal(t, http.StatusOK, rec.Code)

	// 2番目: 制限超過
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetPath("/test")
	_ = handler(c)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)

	// リセットを待つ
	time.Sleep(60 * time.Millisecond)

	// リセット後: 成功
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetPath("/test")
	_ = handler(c)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRateLimiter_DifferentPaths(t *testing.T) {
	rl := NewRateLimiter()
	e := echo.New()

	handler := rl.Limit(RateLimitConfig{
		Duration: time.Minute,
		Max:      1,
	})(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	// パス /a: 成功
	req := httptest.NewRequest(http.MethodGet, "/a", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/a")
	_ = handler(c)
	assert.Equal(t, http.StatusOK, rec.Code)

	// パス /b: 別のカウンターなので成功
	req = httptest.NewRequest(http.MethodGet, "/b", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetPath("/b")
	_ = handler(c)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRateLimiter_RemainingHeader(t *testing.T) {
	rl := NewRateLimiter()
	e := echo.New()

	handler := rl.Limit(RateLimitConfig{
		Duration: time.Minute,
		Max:      5,
	})(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/test")
	_ = handler(c)

	assert.Equal(t, "4", rec.Header().Get("X-RateLimit-Remaining"))
}
