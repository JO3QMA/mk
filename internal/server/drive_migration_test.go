package server

import (
	"testing"

	"github.com/shiroha-a/mk/internal/config"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
)

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
