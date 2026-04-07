package i

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	coreuser "github.com/shiroha-a/mk/internal/core/user"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/server/middleware"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func newTestHandler(t *testing.T) (*Handler, *testutil.MockUserRepository) {
	t.Helper()
	userRepo := testutil.NewMockUserRepository()
	svc := coreuser.NewService(userRepo)
	h := NewHandler(svc)
	return h, userRepo
}

func TestMe_Success(t *testing.T) {
	h, userRepo := newTestHandler(t)

	name := "Test User"
	user := &model.User{
		ID:                "user1",
		Username:          "testuser",
		Name:              &name,
		FollowersCount:    10,
		FollowingCount:    20,
		NotesCount:        100,
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}

	email := "test@example.com"
	userRepo.Profiles["user1"] = &model.UserProfile{
		UserID:             "user1",
		Email:              &email,
		EmailVerified:      true,
		TwoFactorEnabled:   false,
		AutoAcceptFollowed: true,
		NoCrawle:           false,
		PreventAiLearning:  true,
		Fields:             datatypes.JSON([]byte("[]")),
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/i", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(string(middleware.UserContextKey), user)

	err := h.Me(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	assert.Equal(t, "user1", resp["id"])
	assert.Equal(t, "testuser", resp["username"])
	assert.Equal(t, "Test User", resp["name"])
	assert.Equal(t, float64(10), resp["followersCount"])
	assert.Equal(t, float64(20), resp["followingCount"])
	assert.Equal(t, float64(100), resp["notesCount"])

	// Private fields
	assert.Equal(t, "test@example.com", resp["email"])
	assert.Equal(t, true, resp["emailVerified"])
	assert.Equal(t, true, resp["autoAcceptFollowed"])
	assert.Equal(t, false, resp["twoFactorEnabled"])
	assert.Equal(t, true, resp["preventAiLearning"])

	// Hardcoded fields
	assert.Equal(t, false, resp["hasUnreadNotification"])
	assert.Equal(t, false, resp["hasPendingReceivedFollowRequest"])
}

func TestMe_NoProfile(t *testing.T) {
	h, _ := newTestHandler(t)

	user := &model.User{
		ID:                "user1",
		Username:          "noprofile",
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/i", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(string(middleware.UserContextKey), user)

	err := h.Me(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	assert.Equal(t, "user1", resp["id"])
	assert.Equal(t, "noprofile", resp["username"])
	// profileがない場合、private fieldsはレスポンスに含まれない
	assert.Nil(t, resp["email"])
	// ただしhardcoded fieldsは含まれる
	assert.Equal(t, false, resp["hasUnreadNotification"])
}
