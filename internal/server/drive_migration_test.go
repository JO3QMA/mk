package server

import (
	"testing"

	"github.com/shiroha-a/mk/internal/config"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateDriveLocalMigration_Disabled(t *testing.T) {
	cfg := &config.Config{}
	require.NoError(t, validateDriveLocalMigration(cfg, nil))
}

func TestValidateDriveLocalMigration_RequiresMeta(t *testing.T) {
	cfg := &config.Config{
		DriveLocalToObjectStorage: config.DriveLocalToObjectStorageConfig{Enabled: true},
	}
	err := validateDriveLocalMigration(cfg, &model.Meta{UseObjectStorage: false})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "useObjectStorage")
}

func TestValidateDriveLocalMigration_RequiresBucket(t *testing.T) {
	cfg := &config.Config{
		DriveLocalToObjectStorage: config.DriveLocalToObjectStorageConfig{Enabled: true},
	}
	err := validateDriveLocalMigration(cfg, &model.Meta{UseObjectStorage: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "objectStorageBucket")
}

func TestValidateDriveLocalMigration_OK(t *testing.T) {
	bucket := "my-bucket"
	cfg := &config.Config{
		DriveLocalToObjectStorage: config.DriveLocalToObjectStorageConfig{Enabled: true},
	}
	meta := &model.Meta{
		UseObjectStorage:    true,
		ObjectStorageBucket: &bucket,
	}
	require.NoError(t, validateDriveLocalMigration(cfg, meta))
}
