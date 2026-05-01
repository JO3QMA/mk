package admin_test

import (
	"net/http"
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- /admin/unset-user-avatar ---

func TestUnsetUserAvatar(t *testing.T) {
	t.Run("missing userId returns 204 noop", func(t *testing.T) {
		h, _, _, _ := newTestHandler(t)
		assert.Equal(t, http.StatusNoContent, doPost(h.UnsetUserAvatar, `{}`, adminUser).Code)
	})
	t.Run("clears avatarId / avatarUrl / avatarBlurhash", func(t *testing.T) {
		h, repo, _, _ := newTestHandler(t)
		avatarID := "av1"
		avatarURL := "https://x.example/a.webp"
		avatarBlurhash := "blur"
		repo.Users["u1"] = &model.User{
			ID: "u1", AvatarID: &avatarID, AvatarURL: &avatarURL, AvatarBlurhash: &avatarBlurhash,
		}
		assert.Equal(t, http.StatusNoContent, doPost(h.UnsetUserAvatar, `{"userId":"u1"}`, adminUser).Code)
		got := repo.Users["u1"]
		assert.Nil(t, got.AvatarID, "avatarId must be cleared")
		assert.Nil(t, got.AvatarURL, "avatarUrl must be cleared")
		assert.Nil(t, got.AvatarBlurhash, "avatarBlurhash must be cleared")
	})
}

// --- /admin/unset-user-banner ---

func TestUnsetUserBanner(t *testing.T) {
	t.Run("missing userId returns 204 noop", func(t *testing.T) {
		h, _, _, _ := newTestHandler(t)
		assert.Equal(t, http.StatusNoContent, doPost(h.UnsetUserBanner, `{}`, adminUser).Code)
	})
	t.Run("clears bannerId / bannerUrl / bannerBlurhash", func(t *testing.T) {
		h, repo, _, _ := newTestHandler(t)
		bannerID := "b1"
		bannerURL := "https://x.example/b.webp"
		bannerBlurhash := "blur"
		repo.Users["u1"] = &model.User{
			ID: "u1", BannerID: &bannerID, BannerURL: &bannerURL, BannerBlurhash: &bannerBlurhash,
		}
		assert.Equal(t, http.StatusNoContent, doPost(h.UnsetUserBanner, `{"userId":"u1"}`, adminUser).Code)
		got := repo.Users["u1"]
		assert.Nil(t, got.BannerID, "bannerId must be cleared")
		assert.Nil(t, got.BannerURL, "bannerUrl must be cleared")
		assert.Nil(t, got.BannerBlurhash, "bannerBlurhash must be cleared")
	})
}

// --- /admin/update-user-note ---

func TestUpdateUserNote(t *testing.T) {
	t.Run("missing userId returns 204 noop", func(t *testing.T) {
		h, _, _, _ := newTestHandler(t)
		assert.Equal(t, http.StatusNoContent, doPost(h.UpdateUserNote, `{}`, adminUser).Code)
	})
	t.Run("writes moderationNote to user_profile", func(t *testing.T) {
		h, repo, _, _ := newTestHandler(t)
		repo.Profiles["u1"] = &model.UserProfile{UserID: "u1"}
		body := `{"userId":"u1","text":"naughty user"}`
		assert.Equal(t, http.StatusNoContent, doPost(h.UpdateUserNote, body, adminUser).Code)
		got := repo.Profiles["u1"]
		require.NotNil(t, got.ModerationNote)
		assert.Equal(t, "naughty user", *got.ModerationNote)
	})
}
