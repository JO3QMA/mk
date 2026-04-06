package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/misskey-dev/misskey-go/internal/model"
	"github.com/misskey-dev/misskey-go/internal/testutil"
	"github.com/stretchr/testify/assert"
)

func TestAuthenticate_NoToken(t *testing.T) {
	userRepo := testutil.NewMockUserRepository()
	tokenRepo := testutil.NewMockAccessTokenRepository()
	auth := NewAuthMiddleware(userRepo, tokenRepo)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := auth.Authenticate()(func(c echo.Context) error {
		user := GetUser(c)
		assert.Nil(t, user)
		return c.String(http.StatusOK, "ok")
	})

	err := handler(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAuthenticate_BearerToken(t *testing.T) {
	userRepo := testutil.NewMockUserRepository()
	tokenRepo := testutil.NewMockAccessTokenRepository()

	user := &model.User{ID: "user1", Username: "testuser"}
	nativeToken := "abcdef1234567890"
	userRepo.Tokens[nativeToken] = user

	auth := NewAuthMiddleware(userRepo, tokenRepo)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+nativeToken)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := auth.Authenticate()(func(c echo.Context) error {
		u := GetUser(c)
		assert.NotNil(t, u)
		assert.Equal(t, "user1", u.ID)
		return c.String(http.StatusOK, "ok")
	})

	err := handler(c)
	assert.NoError(t, err)
}

func TestAuthenticate_QueryParam(t *testing.T) {
	userRepo := testutil.NewMockUserRepository()
	tokenRepo := testutil.NewMockAccessTokenRepository()

	user := &model.User{ID: "user2", Username: "queryuser"}
	nativeToken := "querytoken123456"
	userRepo.Tokens[nativeToken] = user

	auth := NewAuthMiddleware(userRepo, tokenRepo)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/?i="+nativeToken, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := auth.Authenticate()(func(c echo.Context) error {
		u := GetUser(c)
		assert.NotNil(t, u)
		assert.Equal(t, "user2", u.ID)
		return c.String(http.StatusOK, "ok")
	})

	err := handler(c)
	assert.NoError(t, err)
}

func TestAuthenticate_InvalidToken(t *testing.T) {
	userRepo := testutil.NewMockUserRepository()
	tokenRepo := testutil.NewMockAccessTokenRepository()
	auth := NewAuthMiddleware(userRepo, tokenRepo)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer invalidtoken")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := auth.Authenticate()(func(c echo.Context) error {
		user := GetUser(c)
		// 無効なトークンの場合、ユーザーはnilだがリクエストは継続する
		assert.Nil(t, user)
		return c.String(http.StatusOK, "ok")
	})

	err := handler(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireAuth_Authenticated(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	user := &model.User{ID: "user1", Username: "test"}
	c.Set(string(UserContextKey), user)

	handler := RequireAuth()(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	err := handler(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireAuth_Unauthenticated(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := RequireAuth()(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	err := handler(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestGetUser_NoUser(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	user := GetUser(c)
	assert.Nil(t, user)
}
