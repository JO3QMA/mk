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

// allowlistテストは共有test DBに行を挿入するため、パッケージ外テスト
// (特にinternal/repository/emoji_test.goのListLocal系) を汚染しないよう
// 必ずt.Cleanupで削除する。
func cleanupRow(t *testing.T, db *gorm.DB, table, id string) {
	t.Helper()
	t.Cleanup(func() {
		db.Exec(`DELETE FROM "`+table+`" WHERE id = ?`, id)
	})
}

func TestDBAllowlistChecker_UserAvatarURL(t *testing.T) {
	db := openTestDB(t)
	checker := NewDBAllowlistChecker(db)

	cleanupRow(t, db, "user", "test-allow-u1")
	err := db.Exec(`INSERT INTO "user" (id, "updatedAt", username, "usernameLower", "avatarUrl", token)
		VALUES ('test-allow-u1', NOW(), 'allowtest1', 'allowtest1', 'https://remote.example/avatar-allow.png', 'tok-allow-1')
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

	cleanupRow(t, db, "drive_file", "test-allow-df1")
	cleanupRow(t, db, "user", "test-allow-u2")
	err := db.Exec(`INSERT INTO "user" (id, "updatedAt", username, "usernameLower", token)
		VALUES ('test-allow-u2', NOW(), 'allowtest2', 'allowtest2', 'tok-allow-2')
		ON CONFLICT (id) DO NOTHING`).Error
	require.NoError(t, err)

	err = db.Exec(`INSERT INTO "drive_file" (id, "userId", "userHost", md5, name, type, size, "storedInternal", url, "isSensitive", "isLink")
		VALUES ('test-allow-df1', 'test-allow-u2', NULL, 'aaa', 'test.png', 'image/png', 1024, false, 'https://s3.example/files/allow-test.png', false, false)
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

	cleanupRow(t, db, "emoji", "test-allow-em1")
	// hostをremoteに設定することでlocal emoji扱いから外し、並行実行される
	// internal/repository/emoji_test.goのListLocal系テストに拾われないようにする
	// (Devin review #259: cross-package race指摘)。allowlistはoriginalUrl/publicUrl
	// のみを見てhostは参照しないので、本テストの assert に影響はない。
	err := db.Exec(`INSERT INTO "emoji" (id, "updatedAt", name, host, "originalUrl", "publicUrl", type)
		VALUES ('test-allow-em1', NOW(), 'allowemoji', 'remote.example', 'https://remote.example/emoji/allow.png', 'https://remote.example/emoji/allow.png', 'image/png')
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

	cleanupRow(t, db, "instance", "test-allow-inst1")
	err := db.Exec(`INSERT INTO "instance" (id, host, "firstRetrievedAt")
		VALUES ('test-allow-inst1', 'allow-remote.example', NOW())
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

	cleanupRow(t, db, "user", "test-allow-u3")
	err := db.Exec(`INSERT INTO "user" (id, "updatedAt", username, "usernameLower", "bannerUrl", token)
		VALUES ('test-allow-u3', NOW(), 'allowtest3', 'allowtest3', 'https://remote.example/banner-allow.png', 'tok-allow-3')
		ON CONFLICT (id) DO NOTHING`).Error
	require.NoError(t, err)

	ctx := context.Background()

	ok, err := checker.IsAllowedURL(ctx, "https://remote.example/banner-allow.png")
	assert.NoError(t, err)
	assert.True(t, ok)
}
