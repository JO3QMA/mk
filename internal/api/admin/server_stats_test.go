package admin_test

import (
	"net/http"
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
)

type stubIPRepo struct{}

func (s *stubIPRepo) Upsert(_, _ string) error                            { return nil }
func (s *stubIPRepo) ListByUser(_ string, _ int) ([]*model.UserIP, error) { return nil, nil }

func TestGetIndexStats(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusOK, doPost(h.GetIndexStats, `{}`, adminUser).Code)
}

func TestGetTableStats(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusOK, doPost(h.GetTableStats, `{}`, adminUser).Code)
}

func TestGetUserIPs_NilRepo(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusOK, doPost(h.GetUserIPs, `{}`, adminUser).Code)
}

func TestGetUserIPs_InvalidParam(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetUserIPRepo(&stubIPRepo{})
	assert.Equal(t, http.StatusBadRequest, doPost(h.GetUserIPs, `{}`, adminUser).Code)
}

func TestGetUserIPs_Success(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetUserIPRepo(&stubIPRepo{})
	rec := doPost(h.GetUserIPs, `{"userId":"u1"}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestServerInfo_Disabled(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.ServerInfo, `{}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	// enableServerMachineStats = false (デフォルト) なので Empty() が返る
	assert.Contains(t, rec.Body.String(), `"name":"?"`)
}
