package entity

import (
	"testing"

	"github.com/misskey-dev/misskey-go/internal/model"
	"github.com/stretchr/testify/assert"
	"gorm.io/datatypes"
)

func TestPackUserLite(t *testing.T) {
	name := "Test User"
	avatarURL := "https://example.com/avatar.png"
	blurhash := "LEHV6nWB2yk8"

	u := &model.User{
		ID:                "user1",
		Username:          "testuser",
		Name:              &name,
		AvatarURL:         &avatarURL,
		AvatarBlurhash:    &blurhash,
		AvatarDecorations: datatypes.JSON([]byte("[]")),
		IsBot:             true,
		IsCat:             false,
	}

	lite := PackUserLite(u)

	assert.Equal(t, "user1", lite.ID)
	assert.Equal(t, "testuser", lite.Username)
	assert.Equal(t, &name, lite.Name)
	assert.Equal(t, &avatarURL, lite.AvatarURL)
	assert.Equal(t, &blurhash, lite.AvatarBlurhash)
	assert.True(t, lite.IsBot)
	assert.False(t, lite.IsCat)
	assert.Equal(t, "unknown", lite.OnlineStatus)
	assert.NotNil(t, lite.Emojis)
	assert.Empty(t, lite.Emojis)
	assert.NotNil(t, lite.BadgeRoles)
	assert.Empty(t, lite.BadgeRoles)
}

func TestPackUserLite_NilFields(t *testing.T) {
	u := &model.User{
		ID:                "user2",
		Username:          "minimal",
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}

	lite := PackUserLite(u)

	assert.Equal(t, "user2", lite.ID)
	assert.Nil(t, lite.Name)
	assert.Nil(t, lite.Host)
	assert.Nil(t, lite.AvatarURL)
	assert.Nil(t, lite.AvatarBlurhash)
}

func TestPackUserDetailed(t *testing.T) {
	name := "Detailed User"
	bannerURL := "https://example.com/banner.png"
	desc := "A test user"
	location := "Tokyo"
	birthday := "2000-01-01"
	lang := "ja-JP"

	u := &model.User{
		ID:                "user3",
		Username:          "detailed",
		Name:              &name,
		BannerURL:         &bannerURL,
		IsLocked:          true,
		IsSuspended:       false,
		FollowersCount:    100,
		FollowingCount:    50,
		NotesCount:        1000,
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}

	profile := &model.UserProfile{
		UserID:      "user3",
		Description: &desc,
		Location:    &location,
		Birthday:    &birthday,
		Lang:        &lang,
		Fields:      datatypes.JSON([]byte("[]")),
	}

	detailed := PackUserDetailed(u, profile)

	assert.Equal(t, "user3", detailed.ID)
	assert.Equal(t, "detailed", detailed.Username)
	assert.Equal(t, &bannerURL, detailed.BannerURL)
	assert.True(t, detailed.IsLocked)
	assert.False(t, detailed.IsSuspended)
	assert.Equal(t, 100, detailed.FollowersCount)
	assert.Equal(t, 50, detailed.FollowingCount)
	assert.Equal(t, 1000, detailed.NotesCount)
	assert.Equal(t, &desc, detailed.Description)
	assert.Equal(t, &location, detailed.Location)
	assert.Equal(t, &birthday, detailed.Birthday)
	assert.Equal(t, &lang, detailed.Lang)
}

func TestPackUserDetailed_NilProfile(t *testing.T) {
	u := &model.User{
		ID:                "user4",
		Username:          "noprofile",
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}

	detailed := PackUserDetailed(u, nil)

	assert.Equal(t, "user4", detailed.ID)
	assert.Nil(t, detailed.Description)
	assert.Nil(t, detailed.Location)
	assert.Nil(t, detailed.Birthday)
	assert.Nil(t, detailed.Lang)
}
