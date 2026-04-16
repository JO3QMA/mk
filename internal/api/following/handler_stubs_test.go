package following

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/server/middleware"
	"github.com/stretchr/testify/assert"
)

func postStub(handler func(echo.Context) error, body string, user *model.User) *httptest.ResponseRecorder {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if user != nil {
		c.Set(string(middleware.UserContextKey), user)
	}
	_ = handler(c)
	return rec
}

func TestInvalidate_MissingUserIDRejected(t *testing.T) {
	h, _ := newTestHandler(t)
	assert.Equal(t, http.StatusBadRequest, postStub(h.Invalidate, `{}`, &model.User{ID: "admin"}).Code)
}

func TestUpdateFollow_MissingUserIDRejected(t *testing.T) {
	h, _ := newTestHandler(t)
	assert.Equal(t, http.StatusBadRequest, postStub(h.UpdateFollow, `{}`, &model.User{ID: "u1"}).Code)
}

func TestUpdateFollowAll_NoFieldsNoOp(t *testing.T) {
	h, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, postStub(h.UpdateFollowAll, `{}`, &model.User{ID: "u1"}).Code)
}

func TestRequestsSent_Empty(t *testing.T) {
	h, _ := newTestHandler(t)
	assert.Equal(t, http.StatusOK, postStub(h.RequestsSent, `{}`, &model.User{ID: "u1"}).Code)
}

func TestInvalidate_Success(t *testing.T) {
	h, userRepo := newTestHandler(t)
	// admin に follower を follow させた状態を作る
	admin := &model.User{ID: "admin", Username: "admin", UsernameLower: "admin"}
	follower := &model.User{ID: "follower", Username: "f", UsernameLower: "f"}
	userRepo.Users[admin.ID] = admin
	userRepo.Users[follower.ID] = follower
	_, _ = h.followingService.Follow(follower.ID, admin.ID)

	rec := postStub(h.Invalidate, `{"userId":"follower"}`, admin)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestInvalidate_NotFollowing(t *testing.T) {
	h, userRepo := newTestHandler(t)
	admin := &model.User{ID: "admin", Username: "admin", UsernameLower: "admin"}
	other := &model.User{ID: "other", Username: "o", UsernameLower: "o"}
	userRepo.Users[admin.ID] = admin
	userRepo.Users[other.ID] = other
	// other は admin をフォローしていない → NOT_FOLLOWING
	rec := postStub(h.Invalidate, `{"userId":"other"}`, admin)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUpdateFollow_Success(t *testing.T) {
	h, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent,
		postStub(h.UpdateFollow, `{"userId":"u2","notify":"normal","withReplies":true}`, &model.User{ID: "u1"}).Code)
}

func TestUpdateFollowAll_WithFields(t *testing.T) {
	h, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent,
		postStub(h.UpdateFollowAll, `{"notify":"none"}`, &model.User{ID: "u1"}).Code)
}

func TestRequestsSent_PopulatesFollowerFollowee(t *testing.T) {
	h, userRepo := newTestHandler(t)
	// me が remote-locked を follow リクエストして pending な状態を作る
	me := &model.User{ID: "me", Username: "me", UsernameLower: "me"}
	locked := &model.User{ID: "locked", Username: "locked", UsernameLower: "locked", IsLocked: true}
	userRepo.Users[me.ID] = me
	userRepo.Users[locked.ID] = locked
	_, _ = h.followingService.Follow(me.ID, locked.ID) // ends in FollowRequest

	rec := postStub(h.RequestsSent, `{}`, me)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "me")
}
