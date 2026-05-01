package users

import (
	"net/http"
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"gorm.io/datatypes"
)

func TestAchievements_Success(t *testing.T) {
	h, repo := newTestHandler(t)
	repo.Users["u1"] = &model.User{ID: "u1", Username: "alice"}
	repo.Profiles["u1"] = &model.UserProfile{
		UserID:       "u1",
		Achievements: datatypes.JSON(`[{"name":"notes1","unlockedAt":1000}]`),
	}
	rec := postStub(h.Achievements, `{"userId":"u1"}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "notes1")
}

func TestAchievements_NoProfile(t *testing.T) {
	h, repo := newTestHandler(t)
	repo.Users["u1"] = &model.User{ID: "u1", Username: "alice"}
	rec := postStub(h.Achievements, `{"userId":"u1"}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "[]\n", rec.Body.String())
}

func TestAchievements_NotFound(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := postStub(h.Achievements, `{"userId":"ghost"}`, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAchievements_InvalidParam(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := postStub(h.Achievements, `{}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
