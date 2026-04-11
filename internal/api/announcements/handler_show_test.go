package announcements_test

import (
	"net/http"
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
)

// Show エンドポイントの 3 経路 (bad param / not found / ok) をカバーする。
// 0% だった handler_show.go を完全に exercise する目的。
func TestShow_InvalidParam(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doPost(h.Show, `{}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestShow_NotFound(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doPost(h.Show, `{"announcementId":"ghost"}`, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestShow_Success(t *testing.T) {
	h, repo := newTestHandler(t)
	repo.Items["a1"] = &model.Announcement{ID: "a1", Title: "hi", Text: "hello", IsActive: true}
	rec := doPost(h.Show, `{"announcementId":"a1"}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
}
