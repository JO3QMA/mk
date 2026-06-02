package server

import (
	"context"
	"errors"
	"testing"

	"github.com/shiroha-a/mk/internal/config"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/queue"
	"github.com/shiroha-a/mk/internal/queue/driver"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countSpyFileRepo records whether CountStoredInternal was invoked.
type countSpyFileRepo struct {
	*testutil.MockDriveFileRepository
	countCalled bool
}

func (s *countSpyFileRepo) CountStoredInternal() (int64, error) {
	s.countCalled = true
	return s.MockDriveFileRepository.CountStoredInternal()
}

func TestDriveLocalMigrationReady_Disabled(t *testing.T) {
	cfg := &config.Config{}
	assert.False(t, driveLocalMigrationReady(cfg, nil))
}

func TestDriveLocalMigrationReady_RequiresMeta(t *testing.T) {
	cfg := &config.Config{
		DriveLocalToObjectStorage: config.DriveLocalToObjectStorageConfig{Enabled: true},
	}
	assert.False(t, driveLocalMigrationReady(cfg, &model.Meta{UseObjectStorage: false}))
}

func TestDriveLocalMigrationReady_RequiresBucket(t *testing.T) {
	cfg := &config.Config{
		DriveLocalToObjectStorage: config.DriveLocalToObjectStorageConfig{Enabled: true},
	}
	assert.False(t, driveLocalMigrationReady(cfg, &model.Meta{UseObjectStorage: true}))
}

func TestDriveLocalMigrationReady_OK(t *testing.T) {
	bucket := "my-bucket"
	cfg := &config.Config{
		DriveLocalToObjectStorage: config.DriveLocalToObjectStorageConfig{Enabled: true},
	}
	meta := &model.Meta{
		UseObjectStorage:    true,
		ObjectStorageBucket: &bucket,
	}
	assert.True(t, driveLocalMigrationReady(cfg, meta))
}

func TestMaybeEnqueueDriveLocalMigration_SkipsCountStoredInternal(t *testing.T) {
	bucket := "my-bucket"
	rec := &recordingDriverClient{}
	srv := &Server{
		config: &config.Config{
			DriveLocalToObjectStorage: config.DriveLocalToObjectStorageConfig{Enabled: true},
		},
		queueClient: queue.NewClient(&stubDriver{client: rec}),
	}
	defer func() { _ = srv.queueClient.Close() }()

	spy := &countSpyFileRepo{MockDriveFileRepository: testutil.NewMockDriveFileRepository()}
	meta := &model.Meta{
		UseObjectStorage:    true,
		ObjectStorageBucket: &bucket,
	}

	require.NoError(t, srv.maybeEnqueueDriveLocalMigration(spy, meta))
	assert.False(t, spy.countCalled, "startup must not run COUNT on drive_file")
	assert.Equal(t, queue.TaskTypeObjectStorageMigrateScan, rec.lastTaskType)
}

type failingEnqueueClient struct {
	err error
}

func (f *failingEnqueueClient) Enqueue(context.Context, string, []byte, ...driver.EnqueueOption) error {
	return f.err
}
func (f *failingEnqueueClient) Close() error { return nil }

func TestMaybeEnqueueDriveLocalMigration_EnqueueFailureDoesNotBlockStartup(t *testing.T) {
	bucket := "my-bucket"
	srv := &Server{
		config: &config.Config{
			DriveLocalToObjectStorage: config.DriveLocalToObjectStorageConfig{Enabled: true},
		},
		queueClient: queue.NewClient(&stubDriver{client: &failingEnqueueClient{err: errors.New("redis unavailable")}}),
	}
	defer func() { _ = srv.queueClient.Close() }()

	meta := &model.Meta{
		UseObjectStorage:    true,
		ObjectStorageBucket: &bucket,
	}
	require.NoError(t, srv.maybeEnqueueDriveLocalMigration(testutil.NewMockDriveFileRepository(), meta))
}
