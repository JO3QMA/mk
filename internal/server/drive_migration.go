package server

import (
	"fmt"
	"log/slog"

	"github.com/shiroha-a/mk/internal/config"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

// validateDriveLocalMigration checks meta when YAML migration is enabled.
func validateDriveLocalMigration(cfg *config.Config, meta *model.Meta) error {
	if !cfg.DriveLocalToObjectStorage.Enabled {
		return nil
	}
	if meta == nil || !meta.UseObjectStorage {
		return fmt.Errorf("driveLocalToObjectStorage.enabled requires meta.useObjectStorage=true")
	}
	if meta.ObjectStorageBucket == nil || *meta.ObjectStorageBucket == "" {
		return fmt.Errorf("driveLocalToObjectStorage.enabled requires meta.objectStorageBucket")
	}
	return nil
}

// maybeEnqueueDriveLocalMigration starts the scan job when configured.
func (s *Server) maybeEnqueueDriveLocalMigration(
	fileRepo repository.DriveFileRepository,
) error {
	cfg := s.config.DriveLocalToObjectStorage
	if !cfg.Enabled {
		return nil
	}
	if fileRepo == nil || s.queueClient == nil {
		return fmt.Errorf("drive migration: file repository or queue client not configured")
	}
	n, err := fileRepo.CountStoredInternal()
	if err != nil {
		return fmt.Errorf("drive migration: count storedInternal: %w", err)
	}
	if n == 0 {
		slog.Info("drive local→object storage migration: no pending files")
		return nil
	}
	if err := s.queueClient.EnqueueObjectStorageMigrateScan(); err != nil {
		return fmt.Errorf("drive migration: enqueue scan: %w", err)
	}
	slog.Info("drive local→object storage migration: scan job enqueued", "pendingFiles", n)
	return nil
}
