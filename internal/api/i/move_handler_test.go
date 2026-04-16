package i

import (
	"errors"
	"net/http"
	"testing"

	"github.com/shiroha-a/mk/internal/core/move"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
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
	rec := post(h.Move, `{}`, &model.User{ID: "me"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestMove_NoMover(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	// mover unset
	rec := post(h.Move, `{"moveToAccount":"https://other.example/users/x"}`, &model.User{ID: "me"})
	assert.Equal(t, http.StatusNotImplemented, rec.Code)
}

func TestMove_Success(t *testing.T) {
	h, userRepo, _, _ := newTestHandler(t)
	// Me() が成功するよう最小限の user row を登録する。
	userRepo.Users["me"] = &model.User{ID: "me", Username: "me"}
	sm := &stubMover{}
	h.SetAccountMover(sm)

	rec := post(h.Move, `{"moveToAccount":"https://other.example/users/x"}`, &model.User{ID: "me"})
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, sm.called)
	assert.Equal(t, "https://other.example/users/x", sm.gotURI)
}

func TestMove_NoSuchUser(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetAccountMover(&stubMover{err: move.ErrNoSuchUser})
	rec := post(h.Move, `{"moveToAccount":"https://x"}`, &model.User{ID: "me"})
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "NO_SUCH_USER")
}

func TestMove_AlreadyMoved(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetAccountMover(&stubMover{err: move.ErrAlreadyMoved})
	rec := post(h.Move, `{"moveToAccount":"https://x"}`, &model.User{ID: "me"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "ALREADY_MOVED")
}

func TestMove_DestinationForbids(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetAccountMover(&stubMover{err: move.ErrDestinationForbids})
	rec := post(h.Move, `{"moveToAccount":"https://x"}`, &model.User{ID: "me"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "DESTINATION_ACCOUNT_FORBIDS")
}

func TestMove_RemoteSource(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetAccountMover(&stubMover{err: move.ErrRemoteSourceForbidden})
	rec := post(h.Move, `{"moveToAccount":"https://x"}`, &model.User{ID: "me"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "INVALID_PARAM")
}

func TestMove_UnexpectedError(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetAccountMover(&stubMover{err: errors.New("boom")})
	rec := post(h.Move, `{"moveToAccount":"https://x"}`, &model.User{ID: "me"})
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
