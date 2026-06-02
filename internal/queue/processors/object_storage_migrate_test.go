package processors_test

import (
	"context"
	"errors"
	"testing"

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
