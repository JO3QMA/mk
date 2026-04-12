package mediaproxy

import (
	"context"

	"gorm.io/gorm"
)

// AllowlistChecker determines whether a URL is known in the database.
type AllowlistChecker interface {
	IsAllowedURL(ctx context.Context, url string) (bool, error)
}

// DBAllowlistChecker implements AllowlistChecker by checking URL columns across
// multiple tables. EXISTS + UNION ALL により最初のヒットで短絡する。
type DBAllowlistChecker struct {
	db *gorm.DB
}

// NewDBAllowlistChecker creates an AllowlistChecker backed by the given DB.
func NewDBAllowlistChecker(db *gorm.DB) *DBAllowlistChecker {
	return &DBAllowlistChecker{db: db}
}

// IsAllowedURL returns true if the given URL exists in any of the allowlisted
// columns: user avatar/banner, drive_file url/thumbnail/webpublic/src/uri,
// emoji original/public, instance icon/favicon.
func (c *DBAllowlistChecker) IsAllowedURL(ctx context.Context, url string) (bool, error) {
	var exists bool
	err := c.db.WithContext(ctx).Raw(`
		SELECT EXISTS (
			SELECT 1 FROM "user" WHERE "avatarUrl" = ? OR "bannerUrl" = ?
			UNION ALL
			SELECT 1 FROM "drive_file" WHERE url = ? OR "thumbnailUrl" = ? OR "webpublicUrl" = ? OR src = ? OR uri = ?
			UNION ALL
			SELECT 1 FROM "emoji" WHERE "originalUrl" = ? OR "publicUrl" = ?
			UNION ALL
			SELECT 1 FROM "instance" WHERE "iconUrl" = ? OR "faviconUrl" = ?
		)`,
		url, url,
		url, url, url, url, url,
		url, url,
		url, url,
	).Scan(&exists).Error
	return exists, err
}
