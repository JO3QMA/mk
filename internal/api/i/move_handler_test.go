package i

import (
	"errors"
	"net/http"
	"testing"

	"github.com/shiroha-a/mk/internal/core/move"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

// stubMover lets tests dictate what AccountMover.Move returns.
type stubMover struct {
	err    error
	called bool
	gotSrc *model.User
	gotURI string
}

func (s *stubMover) Move(src *model.User, dstURI string) error {
	s.called = true
	s.gotSrc = src
	s.gotURI = dstURI
	return s.err
}

func TestMove_InvalidParam(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetAccountMover(&stubMover{})
	// moveToAccount 欠落
	assert.Equal(t, http.StatusBadRequest, post(h.Move, `{"password":"pw"}`, &model.User{ID: "me"}).Code)
	// password 欠落
	assert.Equal(t, http.StatusBadRequest, post(h.Move, `{"moveToAccount":"https://x"}`, &model.User{ID: "me"}).Code)
	// 両方欠落
	assert.Equal(t, http.StatusBadRequest, post(h.Move, `{}`, &model.User{ID: "me"}).Code)
}

func TestMove_NoProfile(t *testing.T) {
	h, userRepo, _, _ := newTestHandler(t)
	// user row はあるが profile 未登録 (= password 未設定)
	userRepo.Users["me"] = &model.User{ID: "me"}
	h.SetAccountMover(&stubMover{})
	rec := post(h.Move, `{"moveToAccount":"https://x","password":"pw"}`, &model.User{ID: "me"})
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "ACCESS_DENIED")
}

func TestMove_WrongPassword(t *testing.T) {
	h, userRepo, _, _ := newTestHandler(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct"), bcrypt.MinCost)
	hashStr := string(hash)
	userRepo.Users["me"] = &model.User{ID: "me"}
	userRepo.Profiles["me"] = &model.UserProfile{UserID: "me", Password: &hashStr}
	sm := &stubMover{}
	h.SetAccountMover(sm)
	rec := post(h.Move, `{"moveToAccount":"https://x","password":"wrong"}`, &model.User{ID: "me"})
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "INCORRECT_PASSWORD")
	assert.False(t, sm.called, "パスワード誤りなら Move は呼ばれない")
}

func TestMove_NoMover(t *testing.T) {
	h, userRepo, _, _ := newTestHandler(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("pw"), bcrypt.MinCost)
	hashStr := string(hash)
	userRepo.Users["me"] = &model.User{ID: "me"}
	userRepo.Profiles["me"] = &model.UserProfile{UserID: "me", Password: &hashStr}
	// mover unset
	rec := post(h.Move, `{"moveToAccount":"https://other.example/users/x","password":"pw"}`, &model.User{ID: "me"})
	assert.Equal(t, http.StatusNotImplemented, rec.Code)
}

// moveHandlerWithPasswordUser wires a user + profile + bcrypt hash so the
// password pre-check passes, then returns the authenticated user to pass to
// post(). Body must include "password":"pw".
func moveHandlerWithPasswordUser(t *testing.T) (*Handler, *model.User, func(mover AccountMover)) {
	t.Helper()
	h, userRepo, _, _ := newTestHandler(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("pw"), bcrypt.MinCost)
	hashStr := string(hash)
	user := &model.User{ID: "me", Username: "me"}
	userRepo.Users["me"] = user
	userRepo.Profiles["me"] = &model.UserProfile{UserID: "me", Password: &hashStr}
	return h, user, func(m AccountMover) { h.SetAccountMover(m) }
}

func TestMove_Success(t *testing.T) {
	h, user, setMover := moveHandlerWithPasswordUser(t)
	sm := &stubMover{}
	setMover(sm)

	rec := post(h.Move, `{"moveToAccount":"https://other.example/users/x","password":"pw"}`, user)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, sm.called)
	assert.Equal(t, "https://other.example/users/x", sm.gotURI)
}

func TestMove_NoSuchUser(t *testing.T) {
	h, user, setMover := moveHandlerWithPasswordUser(t)
	setMover(&stubMover{err: move.ErrNoSuchUser})
	rec := post(h.Move, `{"moveToAccount":"https://x","password":"pw"}`, user)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "NO_SUCH_USER")
}

func TestMove_AlreadyMoved(t *testing.T) {
	h, user, setMover := moveHandlerWithPasswordUser(t)
	setMover(&stubMover{err: move.ErrAlreadyMoved})
	rec := post(h.Move, `{"moveToAccount":"https://x","password":"pw"}`, user)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "ALREADY_MOVED")
}

func TestMove_DestinationForbids(t *testing.T) {
	h, user, setMover := moveHandlerWithPasswordUser(t)
	setMover(&stubMover{err: move.ErrDestinationForbids})
	rec := post(h.Move, `{"moveToAccount":"https://x","password":"pw"}`, user)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "DESTINATION_ACCOUNT_FORBIDS")
}

func TestMove_RemoteSource(t *testing.T) {
	h, user, setMover := moveHandlerWithPasswordUser(t)
	setMover(&stubMover{err: move.ErrRemoteSourceForbidden})
	rec := post(h.Move, `{"moveToAccount":"https://x","password":"pw"}`, user)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "INVALID_PARAM")
}

func TestMove_UnexpectedError(t *testing.T) {
	h, user, setMover := moveHandlerWithPasswordUser(t)
	setMover(&stubMover{err: errors.New("boom")})
	rec := post(h.Move, `{"moveToAccount":"https://x","password":"pw"}`, user)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
