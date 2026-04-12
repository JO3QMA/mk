package mediaproxy

import (
	"context"
	"testing"

	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := testutil.OpenTestDB()
	if err != nil {
		t.Skip("test DB not available: " + err.Error())
	}
	testutil.ApplyMigrations(db)
	return db
}

func TestDBAllowlistChecker_UserAvatarURL(t *testing.T) {
	db := openTestDB(t)
	checker := NewDBAllowlistChecker(db)

	err := db.Exec(`INSERT INTO "user" (id, "createdAt", "updatedAt", username, "usernameLower", "avatarUrl", token)
		VALUES ('test-allow-u1', NOW(), NOW(), 'allowtest1', 'allowtest1', 'https://remote.example/avatar-allow.png', 'tok-allow-1')
		ON CONFLICT (id) DO NOTHING`).Error
	require.NoError(t, err)

	ctx := context.Background()

	ok, err := checker.IsAllowedURL(ctx, "https://remote.example/avatar-allow.png")
	assert.NoError(t, err)
	assert.True(t, ok)

	ok, err = checker.IsAllowedURL(ctx, "https://unknown.example/evil.png")
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestDBAllowlistChecker_DriveFileURL(t *testing.T) {
	db := openTestDB(t)
	checker := NewDBAllowlistChecker(db)

	err := db.Exec(`INSERT INTO "user" (id, "createdAt", "updatedAt", username, "usernameLower", token)
		VALUES ('test-allow-u2', NOW(), NOW(), 'allowtest2', 'allowtest2', 'tok-allow-2')
		ON CONFLICT (id) DO NOTHING`).Error
	require.NoError(t, err)

	err = db.Exec(`INSERT INTO "drive_file" (id, "createdAt", "userId", "userHost", md5, name, type, size, url, "bucketId", "isSensitive", "isLink")
		VALUES ('test-allow-df1', NOW(), 'test-allow-u2', NULL, 'aaa', 'test.png', 'image/png', 1024, 'https://s3.example/files/allow-test.png', NULL, false, false)
		ON CONFLICT (id) DO NOTHING`).Error
	require.NoError(t, err)

	ctx := context.Background()

	ok, err := checker.IsAllowedURL(ctx, "https://s3.example/files/allow-test.png")
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestDBAllowlistChecker_EmojiURL(t *testing.T) {
	db := openTestDB(t)
	checker := NewDBAllowlistChecker(db)

	err := db.Exec(`INSERT INTO "emoji" (id, "updatedAt", name, "originalUrl", "publicUrl", type)
		VALUES ('test-allow-em1', NOW(), 'allowemoji', 'https://remote.example/emoji/allow.png', 'https://remote.example/emoji/allow.png', 'image/png')
		ON CONFLICT (id) DO NOTHING`).Error
	require.NoError(t, err)

	ctx := context.Background()

	ok, err := checker.IsAllowedURL(ctx, "https://remote.example/emoji/allow.png")
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestDBAllowlistChecker_InstanceIconURL(t *testing.T) {
	db := openTestDB(t)
	checker := NewDBAllowlistChecker(db)

	err := db.Exec(`INSERT INTO "instance" (id, "caughtAt", host, "firstRetrievedAt")
		VALUES ('test-allow-inst1', NOW(), 'allow-remote.example', NOW())
		ON CONFLICT (id) DO NOTHING`).Error
	require.NoError(t, err)

	err = db.Exec(`UPDATE "instance" SET "iconUrl" = 'https://allow-remote.example/icon.png' WHERE id = 'test-allow-inst1'`).Error
	require.NoError(t, err)

	ctx := context.Background()

	ok, err := checker.IsAllowedURL(ctx, "https://allow-remote.example/icon.png")
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestDBAllowlistChecker_UserBannerURL(t *testing.T) {
	db := openTestDB(t)
	checker := NewDBAllowlistChecker(db)

	err := db.Exec(`INSERT INTO "user" (id, "createdAt", "updatedAt", username, "usernameLower", "bannerUrl", token)
		VALUES ('test-allow-u3', NOW(), NOW(), 'allowtest3', 'allowtest3', 'https://remote.example/banner-allow.png', 'tok-allow-3')
		ON CONFLICT (id) DO NOTHING`).Error
	require.NoError(t, err)

	ctx := context.Background()

	ok, err := checker.IsAllowedURL(ctx, "https://remote.example/banner-allow.png")
	assert.NoError(t, err)
	assert.True(t, ok)
}
