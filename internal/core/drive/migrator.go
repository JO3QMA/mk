package drive

import (
	"context"
	"errors"
	"fmt"

	"github.com/shiroha-a/mk/internal/config"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"gorm.io/gorm"
)

// ErrMigrationNotConfigured is returned when object storage is not enabled in meta.
var ErrMigrationNotConfigured = errors.New("object storage is not configured in meta")

// storageFromMeta is the factory used by MigrateFile; tests may replace it to inject S3Storage.
var storageFromMeta = NewStorageFromMeta

// Migrator moves locally stored drive files into object storage (#1476).
type Migrator struct {
	metaRepo repository.MetaRepository
	fileRepo repository.DriveFileRepository
	db       *gorm.DB
	cfg      config.DriveLocalToObjectStorageConfig
	driveURL string
}

// NewMigrator constructs a Migrator.
func NewMigrator(
	metaRepo repository.MetaRepository,
	fileRepo repository.DriveFileRepository,
	db *gorm.DB,
	cfg config.DriveLocalToObjectStorageConfig,
	driveURL string,
) *Migrator {
	return &Migrator{
		metaRepo: metaRepo,
		fileRepo: fileRepo,
		db:       db,
		cfg:      cfg,
		driveURL: driveURL,
	}
}

// MigrateFile copies one drive_file row from local disk to S3 and updates DB URLs.
func (m *Migrator) MigrateFile(ctx context.Context, fileID string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	meta, err := m.metaRepo.Fetch()
	if err != nil {
		return fmt.Errorf("fetch meta: %w", err)
	}
	if !migrationMetaReady(meta) {
		return ErrMigrationNotConfigured
	}

	f, err := m.fileRepo.FindByID(fileID)
	if err != nil {
		return err
	}
	if !f.StoredInternal || f.IsLink {
		return nil
	}

	local := NewLocalStorage(m.cfg.LocalPath, m.driveURL)
	remote := storageFromMeta(meta, m.cfg.LocalPath, m.driveURL)
	s3, ok := remote.(*S3Storage)
	if !ok {
		return ErrMigrationNotConfigured
	}

	oldURL := f.URL
	oldThumbURL := f.ThumbnailURL
	oldWebpublicURL := f.WebpublicURL

	newURL, err := m.migrateObject(local, s3, f.AccessKey)
	if err != nil {
		return fmt.Errorf("migrate primary %s: %w", fileID, err)
	}

	var newThumbURL, newWebpublicURL *string
	if f.ThumbnailAccessKey != nil {
		u, err := m.migrateObject(local, s3, f.ThumbnailAccessKey)
		if err != nil {
			return fmt.Errorf("migrate thumbnail %s: %w", fileID, err)
		}
		newThumbURL = &u
	}
	if f.WebpublicAccessKey != nil {
		u, err := m.migrateObject(local, s3, f.WebpublicAccessKey)
		if err != nil {
			return fmt.Errorf("migrate webpublic %s: %w", fileID, err)
		}
		newWebpublicURL = &u
	}

	fields := map[string]any{
		"url":            newURL,
		"storedInternal": false,
	}
	if newThumbURL != nil {
		fields["thumbnailUrl"] = *newThumbURL
	}
	if newWebpublicURL != nil {
		fields["webpublicUrl"] = *newWebpublicURL
	}

	if err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.DriveFile{}).Where("id = ?", fileID).Updates(fields).Error; err != nil {
			return err
		}
		return cascadeDenormalizedURLs(tx, fileID, oldURL, newURL, oldThumbURL, newThumbURL, oldWebpublicURL, newWebpublicURL)
	}); err != nil {
		return err
	}

	if m.cfg.DeleteLocal {
		m.deleteLocalKeys(local, f)
	}
	return nil
}

func migrationMetaReady(meta *model.Meta) bool {
	if meta == nil || !meta.UseObjectStorage {
		return false
	}
	if meta.ObjectStorageBucket == nil || *meta.ObjectStorageBucket == "" {
		return false
	}
	return true
}

func (m *Migrator) migrateObject(local Storage, s3 *S3Storage, accessKey *string) (string, error) {
	if accessKey == nil || *accessKey == "" {
		return "", fmt.Errorf("empty access key")
	}
	key := *accessKey
	body, err := local.Get(key)
	if err != nil {
		if errors.Is(err, ErrObjectNotFound) {
			if _, headErr := s3.Get(key); headErr == nil {
				return s3.publicURL(key), nil
			}
			return "", fmt.Errorf("local object missing for key %s: %w", key, err)
		}
		return "", err
	}
	defer body.Close()
	return s3.Put(key, body)
}

func (m *Migrator) deleteLocalKeys(local Storage, f *model.DriveFile) {
	if f.AccessKey != nil {
		_ = local.Delete(*f.AccessKey)
	}
	if f.ThumbnailAccessKey != nil {
		_ = local.Delete(*f.ThumbnailAccessKey)
	}
	if f.WebpublicAccessKey != nil {
		_ = local.Delete(*f.WebpublicAccessKey)
	}
}

func cascadeDenormalizedURLs(
	tx *gorm.DB,
	fileID, oldURL, newURL string,
	oldThumbURL, newThumbURL *string,
	oldWebpublicURL, newWebpublicURL *string,
) error {
	if err := tx.Model(&model.User{}).Where(`"avatarId" = ?`, fileID).Update("avatarUrl", newURL).Error; err != nil {
		return err
	}
	if err := tx.Model(&model.User{}).Where(`"bannerId" = ?`, fileID).Update("bannerUrl", newURL).Error; err != nil {
		return err
	}
	if oldURL != "" {
		if err := tx.Model(&model.Emoji{}).Where(`"originalUrl" = ?`, oldURL).Update("originalUrl", newURL).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Emoji{}).Where(`"publicUrl" = ?`, oldURL).Update("publicUrl", newURL).Error; err != nil {
			return err
		}
	}
	if oldThumbURL != nil && newThumbURL != nil && *oldThumbURL != "" {
		if err := tx.Model(&model.Emoji{}).Where(`"originalUrl" = ?`, *oldThumbURL).Update("originalUrl", *newThumbURL).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Emoji{}).Where(`"publicUrl" = ?`, *oldThumbURL).Update("publicUrl", *newThumbURL).Error; err != nil {
			return err
		}
	}
	if oldWebpublicURL != nil && newWebpublicURL != nil && *oldWebpublicURL != "" {
		if err := tx.Model(&model.Emoji{}).Where(`"originalUrl" = ?`, *oldWebpublicURL).Update("originalUrl", *newWebpublicURL).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Emoji{}).Where(`"publicUrl" = ?`, *oldWebpublicURL).Update("publicUrl", *newWebpublicURL).Error; err != nil {
			return err
		}
	}
	return nil
}
