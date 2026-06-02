package drive

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/shiroha-a/mk/internal/config"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestMigrationMetaReady(t *testing.T) {
	bucket := "b"
	assert.False(t, migrationMetaReady(nil))
	assert.False(t, migrationMetaReady(&model.Meta{UseObjectStorage: false}))
	assert.False(t, migrationMetaReady(&model.Meta{UseObjectStorage: true}))
	assert.True(t, migrationMetaReady(&model.Meta{
		UseObjectStorage:    true,
		ObjectStorageBucket: &bucket,
	}))
}

func TestMigrator_MigrateObject_FromLocal(t *testing.T) {
	dir := t.TempDir()
	key := "abc123"
	require.NoError(t, os.WriteFile(filepath.Join(dir, key), []byte("hello"), 0o644))

	mock := &mockS3API{}
	s3 := NewS3Storage(S3StorageConfig{
		Client:  mock,
		Bucket:  "bucket",
		Prefix:  "files",
		BaseURL: "https://cdn.example.com",
	})
	local := NewLocalStorage(dir, "https://example.com/files")

	m := &Migrator{}
	url, err := m.migrateObject(local, s3, &key)
	require.NoError(t, err)
	assert.Equal(t, "https://cdn.example.com/files/abc123", url)
	require.NotNil(t, mock.putInput)
}

func TestMigrator_MigrateObject_FallbackWhenLocalMissing(t *testing.T) {
	dir := t.TempDir()
	key := "only-s3"
	mock := &mockS3API{getBody: io.NopCloser(bytes.NewReader([]byte("x")))}
	s3 := NewS3Storage(S3StorageConfig{
		Client:  mock,
		Bucket:  "bucket",
		BaseURL: "https://cdn.example.com",
	})
	local := NewLocalStorage(dir, "https://example.com/files")

	m := &Migrator{}
	url, err := m.migrateObject(local, s3, &key)
	require.NoError(t, err)
	assert.Equal(t, "https://cdn.example.com/only-s3", url)
}

func TestMigrator_MigrateObject_EmptyKey(t *testing.T) {
	m := &Migrator{}
	_, err := m.migrateObject(NewLocalStorage(t.TempDir(), ""), &S3Storage{}, nil)
	require.Error(t, err)
}

func TestMigrator_DeleteLocalKeys(t *testing.T) {
	dir := t.TempDir()
	key := "k1"
	require.NoError(t, os.WriteFile(filepath.Join(dir, key), []byte("x"), 0o644))
	local := NewLocalStorage(dir, "")
	m := &Migrator{}
	f := &model.DriveFile{AccessKey: &key}
	m.deleteLocalKeys(local, f)
	_, err := os.Stat(filepath.Join(dir, key))
	assert.True(t, os.IsNotExist(err))
}

func TestMigrator_MigrateFile_SkipsNonInternal(t *testing.T) {
	bucket := "b"
	metaRepo := testutil.NewMockMetaRepository()
	metaRepo.Meta = &model.Meta{UseObjectStorage: true, ObjectStorageBucket: &bucket}
	fileRepo := testutil.NewMockDriveFileRepository()
	fileRepo.Files["f1"] = &model.DriveFile{ID: "f1", StoredInternal: false, IsLink: false}
	m := NewMigrator(metaRepo, fileRepo, nil, config.DriveLocalToObjectStorageConfig{}, "https://example.com/files")
	require.NoError(t, m.MigrateFile(context.Background(), "f1"))
}

func TestMigrator_MigrateFile_NotConfigured(t *testing.T) {
	metaRepo := testutil.NewMockMetaRepository()
	metaRepo.Meta = &model.Meta{UseObjectStorage: false}
	fileRepo := testutil.NewMockDriveFileRepository()
	key := "k"
	fileRepo.Files["f1"] = &model.DriveFile{ID: "f1", StoredInternal: true, AccessKey: &key}
	m := NewMigrator(metaRepo, fileRepo, nil, config.DriveLocalToObjectStorageConfig{}, "https://example.com/files")
	err := m.MigrateFile(context.Background(), "f1")
	require.ErrorIs(t, err, ErrMigrationNotConfigured)
}

func TestMigrator_MigrateFile_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m := NewMigrator(testutil.NewMockMetaRepository(), testutil.NewMockDriveFileRepository(), nil, config.DriveLocalToObjectStorageConfig{}, "")
	err := m.MigrateFile(ctx, "f1")
	require.ErrorIs(t, err, context.Canceled)
}

func TestMigrator_MigrateFile_SkipsLink(t *testing.T) {
	bucket := "b"
	metaRepo := testutil.NewMockMetaRepository()
	metaRepo.Meta = &model.Meta{UseObjectStorage: true, ObjectStorageBucket: &bucket}
	fileRepo := testutil.NewMockDriveFileRepository()
	key := "k"
	fileRepo.Files["f1"] = &model.DriveFile{ID: "f1", StoredInternal: true, IsLink: true, AccessKey: &key}
	m := NewMigrator(metaRepo, fileRepo, nil, config.DriveLocalToObjectStorageConfig{}, "https://example.com/files")
	require.NoError(t, m.MigrateFile(context.Background(), "f1"))
}

func TestMigrator_MigrateObject_LocalMissingNoS3(t *testing.T) {
	key := "missing"
	local := NewLocalStorage(t.TempDir(), "https://example.com/files")
	s3 := NewS3Storage(S3StorageConfig{Client: &mockS3API{getErr: errors.New("not found")}, Bucket: "b", BaseURL: "https://cdn.example.com"})
	m := &Migrator{}
	_, err := m.migrateObject(local, s3, &key)
	require.Error(t, err)
}

func TestMigrator_DeleteLocalKeys_AllVariants(t *testing.T) {
	dir := t.TempDir()
	keys := []string{"p", "t", "w"}
	for _, k := range keys {
		require.NoError(t, os.WriteFile(filepath.Join(dir, k), []byte("x"), 0o644))
	}
	local := NewLocalStorage(dir, "")
	m := &Migrator{}
	f := &model.DriveFile{
		AccessKey:          &keys[0],
		ThumbnailAccessKey: &keys[1],
		WebpublicAccessKey: &keys[2],
	}
	m.deleteLocalKeys(local, f)
	for _, k := range keys {
		_, err := os.Stat(filepath.Join(dir, k))
		assert.True(t, os.IsNotExist(err))
	}
}

func TestMigrator_MigrateFile_HappyPath(t *testing.T) {
	db := openMigratorTestDB(t)

	dir := t.TempDir()
	key := "migkey"
	require.NoError(t, os.WriteFile(filepath.Join(dir, key), []byte("payload"), 0o644))

	bucket := "bucket"
	baseURL := "https://cdn.example.com"
	meta := &model.Meta{UseObjectStorage: true, ObjectStorageBucket: &bucket, ObjectStorageBaseURL: &baseURL}
	metaRepo := testutil.NewMockMetaRepository()
	metaRepo.Meta = meta

	oldURL := "https://example.com/files/" + key
	f := &model.DriveFile{
		ID:             "fmig_happy",
		StoredInternal: true,
		AccessKey:      &key,
		URL:            oldURL,
		Properties:     datatypes.JSON([]byte("{}")),
		RequestHeaders: datatypes.JSON([]byte("{}")),
	}
	require.NoError(t, db.Create(f).Error)
	t.Cleanup(func() { db.Exec(`DELETE FROM "drive_file" WHERE id = ?`, f.ID) })

	fileRepo := testutil.NewMockDriveFileRepository()
	fileRepo.Files[f.ID] = f

	mock := &mockS3API{}
	restore := setStorageFromMetaForTest(func(_ *model.Meta, localDir, _ string) Storage {
		return NewS3Storage(S3StorageConfig{
			Client:  mock,
			Bucket:  bucket,
			Prefix:  "files",
			BaseURL: baseURL,
		})
	})
	t.Cleanup(restore)

	m := NewMigrator(metaRepo, fileRepo, db, config.DriveLocalToObjectStorageConfig{
		LocalPath:   dir,
		DeleteLocal: true,
	}, "https://example.com/files")
	require.NoError(t, m.MigrateFile(context.Background(), f.ID))

	var got model.DriveFile
	require.NoError(t, db.First(&got, "id = ?", f.ID).Error)
	assert.False(t, got.StoredInternal)
	assert.Equal(t, "https://cdn.example.com/files/migkey", got.URL)
	_, err := os.Stat(filepath.Join(dir, key))
	assert.True(t, os.IsNotExist(err))
}

func TestCascadeDenormalizedURLs(t *testing.T) {
	db := openMigratorTestDB(t)

	fileID := "fcascade1"
	oldURL := "https://example.com/files/cascade1"
	newURL := "https://cdn.example.com/files/cascade1"
	oldThumb := "https://example.com/files/cascade1-thumb"
	newThumb := "https://cdn.example.com/files/cascade1-thumb"

	avatarFile := &model.DriveFile{
		ID:             fileID,
		MD5:            "cascade_md5",
		Name:           "cascade.bin",
		Type:           "application/octet-stream",
		Size:           1,
		StoredInternal: true,
		URL:            oldURL,
		Properties:     datatypes.JSON([]byte("{}")),
		RequestHeaders: datatypes.JSON([]byte("{}")),
	}
	require.NoError(t, db.Create(avatarFile).Error)
	t.Cleanup(func() { db.Exec(`DELETE FROM "drive_file" WHERE id = ?`, fileID) })

	user := &model.User{
		ID:                "ucascade1",
		Username:          "cascade1",
		UsernameLower:     "cascade1",
		AvatarID:          &fileID,
		AvatarURL:         &oldURL,
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}
	require.NoError(t, db.Create(user).Error)
	t.Cleanup(func() { db.Exec(`DELETE FROM "user" WHERE id = ?`, user.ID) })

	emoji := &model.Emoji{
		ID:          "ecascade1",
		Name:        "cascade_emoji",
		OriginalURL: oldURL,
		PublicURL:   oldURL,
	}
	require.NoError(t, db.Create(emoji).Error)
	t.Cleanup(func() { db.Exec(`DELETE FROM "emoji" WHERE id = ?`, emoji.ID) })

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return cascadeDenormalizedURLs(tx, fileID, oldURL, newURL, &oldThumb, &newThumb, nil, nil)
	}))

	var gotUser model.User
	require.NoError(t, db.First(&gotUser, "id = ?", user.ID).Error)
	require.NotNil(t, gotUser.AvatarURL)
	assert.Equal(t, newURL, *gotUser.AvatarURL)

	var gotEmoji model.Emoji
	require.NoError(t, db.First(&gotEmoji, "id = ?", emoji.ID).Error)
	assert.Equal(t, newURL, gotEmoji.OriginalURL)
	assert.Equal(t, newURL, gotEmoji.PublicURL)
}

func openMigratorTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := testutil.OpenTestDB()
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	testutil.ApplyMigrations(db)
	return db
}

func setStorageFromMetaForTest(fn func(*model.Meta, string, string) Storage) func() {
	prev := storageFromMeta
	storageFromMeta = fn
	return func() { storageFromMeta = prev }
}
