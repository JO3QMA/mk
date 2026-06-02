package server

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/shiroha-a/mk/internal/config"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/queue/processors"
	"github.com/shiroha-a/mk/internal/repository"
)

// driveLocalMigrationReady reports whether background migration may run.
// When YAML enables migration but meta is not configured for S3, logs a warning
// and returns false so the server still starts (#1476 review).
func driveLocalMigrationReady(cfg *config.Config, meta *model.Meta) bool {
	if cfg == nil || !cfg.DriveLocalToObjectStorage.Enabled {
		return false
	}
	if meta == nil || !meta.UseObjectStorage {
		slog.Warn("driveLocalToObjectStorage.enabled is set but meta.useObjectStorage is false; migration jobs will not run")
		return false
	}
	if meta.ObjectStorageBucket == nil || *meta.ObjectStorageBucket == "" {
		slog.Warn("driveLocalToObjectStorage.enabled is set but meta.objectStorageBucket is empty; migration jobs will not run")
		return false
	}
	return true
}

// maybeEnqueueDriveLocalMigration starts the scan job when configured.
func (s *Server) maybeEnqueueDriveLocalMigration(
	fileRepo repository.DriveFileRepository,
	meta *model.Meta,
) error {
	if !driveLocalMigrationReady(s.config, meta) {
		return nil
	}
	if fileRepo == nil || s.queueClient == nil {
		return fmt.Errorf("drive migration: file repository or queue client not configured")
	}
	if s.redis != nil && s.redis.JobQueue != nil {
		ctx := context.Background()
		held, err := s.redis.JobQueue.Exists(ctx, processors.ObjectStorageScanLockKey).Result()
		if err != nil {
			return fmt.Errorf("drive migration: scan lock check: %w", err)
		}
		if held > 0 {
			slog.Info("drive local→object storage migration: scan already scheduled or running")
			return nil
		}
	}
	if err := s.queueClient.EnqueueObjectStorageMigrateScan(); err != nil {
		return fmt.Errorf("drive migration: enqueue scan: %w", err)
	}
	slog.Info("drive local→object storage migration: scan job enqueued")
	return nil
}
