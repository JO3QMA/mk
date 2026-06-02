package processors_test

import (
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/shiroha-a/mk/internal/config"
	coredrive "github.com/shiroha-a/mk/internal/core/drive"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/queue"
	"github.com/shiroha-a/mk/internal/queue/driver"
	"github.com/shiroha-a/mk/internal/queue/processors"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func TestObjectStorageMigrateProcessor_HandleFile_NotConfigured(t *testing.T) {
	p := processors.NewObjectStorageMigrateProcessor(processors.ObjectStorageMigrateProcessorConfig{})
	task := queue.NewObjectStorageMigrateFileTask(queue.ObjectStorageMigrateFilePayload{FileID: "f1"})
	err := p.Handle(context.Background(), task)
	require.Error(t, err)
	require.ErrorIs(t, err, driver.SkipRetry)
}

func TestObjectStorageMigrateProcessor_HandleFile_MissingID(t *testing.T) {
	p := processors.NewObjectStorageMigrateProcessor(processors.ObjectStorageMigrateProcessorConfig{
		Migrator: coredrive.NewMigrator(nil, nil, nil, config.DriveLocalToObjectStorageConfig{}, "https://example.com/files"),
	})
	task := queue.NewObjectStorageMigrateFileTask(queue.ObjectStorageMigrateFilePayload{})
	err := p.Handle(context.Background(), task)
	require.Error(t, err)
	require.ErrorIs(t, err, driver.SkipRetry)
}

func TestObjectStorageMigrateProcessor_HandleScan_Enqueues(t *testing.T) {
	key := "abc"
	repo := testutil.NewMockDriveFileRepository()
	repo.Files["a"] = &model.DriveFile{
		ID:             "a",
		StoredInternal: true,
		URL:            "https://example.com/files/" + key,
		AccessKey:      &key,
		Properties:     datatypes.JSON([]byte("{}")),
	}
	var enqueued []string
	p := processors.NewObjectStorageMigrateProcessor(processors.ObjectStorageMigrateProcessorConfig{
		FileRepo: repo,
		EnqueueFile: func(fileID string) error {
			enqueued = append(enqueued, fileID)
			return nil
		},
	})
	err := p.Handle(context.Background(), queue.NewObjectStorageMigrateScanTask())
	require.NoError(t, err)
	require.Contains(t, enqueued, "a")
}

func TestObjectStorageMigrateProcessor_HandleFile_MigrationNotConfigured(t *testing.T) {
	meta := testutil.NewMockMetaRepository()
	meta.Meta = &model.Meta{UseObjectStorage: false}
	fileRepo := testutil.NewMockDriveFileRepository()
	key := "k"
	fileRepo.Files["f1"] = &model.DriveFile{
		ID:             "f1",
		StoredInternal: true,
		URL:            "https://example.com/files/k",
		AccessKey:      &key,
		Properties:     datatypes.JSON([]byte("{}")),
	}
	m := coredrive.NewMigrator(meta, fileRepo, nil, config.DriveLocalToObjectStorageConfig{}, "https://example.com/files")
	p := processors.NewObjectStorageMigrateProcessor(processors.ObjectStorageMigrateProcessorConfig{Migrator: m})
	task := queue.NewObjectStorageMigrateFileTask(queue.ObjectStorageMigrateFilePayload{FileID: "f1"})
	err := p.Handle(context.Background(), task)
	require.Error(t, err)
	require.ErrorIs(t, err, driver.SkipRetry)
	require.True(t, errors.Is(err, coredrive.ErrMigrationNotConfigured))
}

func TestObjectStorageMigrateProcessor_HandleScan_NotConfigured(t *testing.T) {
	p := processors.NewObjectStorageMigrateProcessor(processors.ObjectStorageMigrateProcessorConfig{})
	err := p.Handle(context.Background(), queue.NewObjectStorageMigrateScanTask())
	require.Error(t, err)
	require.ErrorIs(t, err, driver.SkipRetry)
}

func TestObjectStorageMigrateProcessor_Handle_UnknownType(t *testing.T) {
	p := processors.NewObjectStorageMigrateProcessor(processors.ObjectStorageMigrateProcessorConfig{})
	err := p.Handle(context.Background(), driver.RawTask{TypeName: "objectStorage:unknown", Body: []byte("{}")})
	require.Error(t, err)
	require.ErrorIs(t, err, driver.SkipRetry)
}

func TestObjectStorageMigrateProcessor_HandleFile_BadPayload(t *testing.T) {
	p := processors.NewObjectStorageMigrateProcessor(processors.ObjectStorageMigrateProcessorConfig{
		Migrator: coredrive.NewMigrator(nil, nil, nil, config.DriveLocalToObjectStorageConfig{}, "https://example.com/files"),
	})
	err := p.Handle(context.Background(), driver.RawTask{TypeName: queue.TaskTypeObjectStorageMigrateFile, Body: []byte("not-json")})
	require.Error(t, err)
	require.ErrorIs(t, err, driver.SkipRetry)
}

func TestObjectStorageMigrateProcessor_HandleScan_LockHeld(t *testing.T) {
	mr, rdb := newMiniredisClient(t)
	require.NoError(t, mr.Set("mk:drive:objectStorage:migrateScan:lock", "1"))

	repo := testutil.NewMockDriveFileRepository()
	p := processors.NewObjectStorageMigrateProcessor(processors.ObjectStorageMigrateProcessorConfig{
		FileRepo: repo,
		EnqueueFile: func(string) error {
			t.Fatal("enqueue should not run when lock held")
			return nil
		},
		Redis: rdb,
	})
	require.NoError(t, p.Handle(context.Background(), queue.NewObjectStorageMigrateScanTask()))
}

func TestObjectStorageMigrateProcessor_HandleScan_WithRedisLock(t *testing.T) {
	_, rdb := newMiniredisClient(t)

	key := "k"
	repo := testutil.NewMockDriveFileRepository()
	repo.Files["a"] = &model.DriveFile{
		ID:             "a",
		StoredInternal: true,
		URL:            "https://example.com/files/k",
		AccessKey:      &key,
		Properties:     datatypes.JSON([]byte("{}")),
	}
	var enqueued []string
	p := processors.NewObjectStorageMigrateProcessor(processors.ObjectStorageMigrateProcessorConfig{
		FileRepo: repo,
		EnqueueFile: func(fileID string) error {
			enqueued = append(enqueued, fileID)
			return nil
		},
		Redis: rdb,
	})
	require.NoError(t, p.Handle(context.Background(), queue.NewObjectStorageMigrateScanTask()))
	require.Contains(t, enqueued, "a")
}

func newMiniredisClient(t *testing.T) (*miniredis.Miniredis, goredis.UniversalClient) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, rdb
}
