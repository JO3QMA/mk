package signin_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/signin"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func newTestHandler(t *testing.T) (*signin.Handler, *testutil.MockUserRepository) {
	t.Helper()
	userRepo := testutil.NewMockUserRepository()
	return signin.NewHandler(userRepo), userRepo
}

func doPost(h func(echo.Context) error, body string) *httptest.ResponseRecorder {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/signin", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	_ = h(c)
	return rec
}

func createTestUser(repo *testutil.MockUserRepository, username, password string) *model.User {
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	token := "testtoken1234567"
	user := &model.User{
		ID:            "u1",
		Username:      username,
		UsernameLower: strings.ToLower(username),
		Token:         &token,
	}
	repo.Users["u1"] = user
	hashStr := string(hash)
	repo.Profiles["u1"] = &model.UserProfile{
		UserID:   "u1",
		Password: &hashStr,
	}
	return user
}

func TestSignin_Step1_NoPassword(t *testing.T) {
	h, repo := newTestHandler(t)
	createTestUser(repo, "admin", "pass123")

	rec := doPost(h.Signin, `{"username":"admin"}`)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, false, resp["finished"])
	assert.Equal(t, "password", resp["next"])
}

func TestSignin_Step2_Success(t *testing.T) {
	h, repo := newTestHandler(t)
	createTestUser(repo, "admin", "pass123")

	rec := doPost(h.Signin, `{"username":"admin","password":"pass123"}`)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["finished"])
	assert.Equal(t, "u1", resp["id"])
	assert.Equal(t, "testtoken1234567", resp["i"])
}

func TestSignin_WrongPassword(t *testing.T) {
	h, repo := newTestHandler(t)
	createTestUser(repo, "admin", "pass123")

	rec := doPost(h.Signin, `{"username":"admin","password":"wrongpass"}`)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestSignin_UserNotFound(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doPost(h.Signin, `{"username":"ghost","password":"x"}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestSignin_EmptyUsername(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doPost(h.Signin, `{}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSignin_SuspendedUser(t *testing.T) {
	h, repo := newTestHandler(t)
	user := createTestUser(repo, "banned", "pass")
	user.IsSuspended = true

	rec := doPost(h.Signin, `{"username":"banned","password":"pass"}`)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestSignin_NoProfile(t *testing.T) {
	h, repo := newTestHandler(t)
	repo.Users["u1"] = &model.User{
		ID: "u1", Username: "noprof", UsernameLower: "noprof",
	}
	// プロフィールなし

	rec := doPost(h.Signin, `{"username":"noprof","password":"x"}`)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestSignin_NilToken(t *testing.T) {
	h, repo := newTestHandler(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.MinCost)
	hashStr := string(hash)
	repo.Users["u1"] = &model.User{
		ID: "u1", Username: "notoken", UsernameLower: "notoken",
		Token: nil, // トークンなし
	}
	repo.Profiles["u1"] = &model.UserProfile{UserID: "u1", Password: &hashStr}

	rec := doPost(h.Signin, `{"username":"notoken","password":"pass"}`)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "", resp["i"]) // token is empty string
}
