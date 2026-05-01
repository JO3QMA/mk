package admin_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
)

// fakeRelaySvc records calls + returns configurable results for the
// admin/relays handlers.
type fakeRelaySvc struct {
	added   []string
	removed []string
	listed  int
	retRel  *model.Relay
	retList []*model.Relay
	err     error
}

func (f *fakeRelaySvc) Add(_ context.Context, inbox string) (*model.Relay, error) {
	f.added = append(f.added, inbox)
	return f.retRel, f.err
}

func (f *fakeRelaySvc) Remove(_ context.Context, id string) error {
	f.removed = append(f.removed, id)
	return f.err
}

func (f *fakeRelaySvc) List(_ context.Context) ([]*model.Relay, error) {
	f.listed++
	return f.retList, f.err
}

func TestRelaysAdd(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.RelaysAdd, `{}`, adminUser).Code)
}

func TestRelaysAdd_WithService(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	svc := &fakeRelaySvc{retRel: &model.Relay{ID: "rel1", Inbox: "https://r.example/inbox", Status: "requesting"}}
	h.SetRelayService(svc)
	rec := doPost(h.RelaysAdd, `{"inbox":"https://r.example/inbox"}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, []string{"https://r.example/inbox"}, svc.added)
}

func TestRelaysAdd_ServiceError(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	svc := &fakeRelaySvc{err: errors.New("boom")}
	h.SetRelayService(svc)
	rec := doPost(h.RelaysAdd, `{"inbox":"https://r.example/inbox"}`, adminUser)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestRelaysList(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusOK, doPost(h.RelaysList, `{}`, adminUser).Code)
}

func TestRelaysList_WithService(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	svc := &fakeRelaySvc{retList: []*model.Relay{{ID: "a"}, {ID: "b"}}}
	h.SetRelayService(svc)
	rec := doPost(h.RelaysList, `{}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 1, svc.listed)
}

func TestRelaysList_WithService_NilSlice(t *testing.T) {
	// list が nil を返しても [] を返す (本家互換)
	h, _, _, _ := newTestHandler(t)
	svc := &fakeRelaySvc{retList: nil}
	h.SetRelayService(svc)
	rec := doPost(h.RelaysList, `{}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "[]")
}

func TestRelaysRemove(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.RelaysRemove, `{}`, adminUser).Code)
}

func TestRelaysRemove_WithService(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	svc := &fakeRelaySvc{}
	h.SetRelayService(svc)
	rec := doPost(h.RelaysRemove, `{"id":"rel1"}`, adminUser)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, []string{"rel1"}, svc.removed)
}

func TestRelaysRemove_WithService_NoID(t *testing.T) {
	// id も inbox も無い場合はスキップして 204 を返す
	h, _, _, _ := newTestHandler(t)
	svc := &fakeRelaySvc{}
	h.SetRelayService(svc)
	rec := doPost(h.RelaysRemove, `{}`, adminUser)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, svc.removed)
}
