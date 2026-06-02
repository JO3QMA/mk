package processors

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	coredrive "github.com/shiroha-a/mk/internal/core/drive"
	"github.com/shiroha-a/mk/internal/queue"
	"github.com/shiroha-a/mk/internal/queue/driver"
	"github.com/shiroha-a/mk/internal/repository"
)

const (
	objectStorageScanLockKey   = "mk:drive:objectStorage:migrateScan:lock"
	objectStorageScanLockTTL   = 6 * time.Hour
	objectStorageScanBatchSize = 500
)

// ObjectStorageMigrateProcessor handles local→S3 drive migration tasks (#1476).
type ObjectStorageMigrateProcessor struct {
	migrator    *coredrive.Migrator
	fileRepo    repository.DriveFileRepository
	enqueueFile func(fileID string) error
	redis       redis.UniversalClient
}

// ObjectStorageMigrateProcessorConfig wires dependencies for the processor.
type ObjectStorageMigrateProcessorConfig struct {
	Migrator    *coredrive.Migrator
	FileRepo    repository.DriveFileRepository
	EnqueueFile func(fileID string) error
	Redis       redis.UniversalClient
}

// NewObjectStorageMigrateProcessor constructs the processor.
func NewObjectStorageMigrateProcessor(cfg ObjectStorageMigrateProcessorConfig) *ObjectStorageMigrateProcessor {
	return &ObjectStorageMigrateProcessor{
		migrator:    cfg.Migrator,
		fileRepo:    cfg.FileRepo,
		enqueueFile: cfg.EnqueueFile,
		redis:       cfg.Redis,
	}
}

// HandleScan fans out migrateFile jobs for storedInternal rows.
func (p *ObjectStorageMigrateProcessor) HandleScan(ctx context.Context) error {
	if p.fileRepo == nil || p.enqueueFile == nil {
		return fmt.Errorf("object storage migrate scan not configured: %w", driver.SkipRetry)
	}
	if p.redis != nil {
		ok, err := p.redis.SetNX(ctx, objectStorageScanLockKey, "1", objectStorageScanLockTTL).Result()
		if err != nil {
			return err
		}
		if !ok {
			slog.Info("object storage migrate scan skipped: lock held")
			return nil
		}
		defer func() { _ = p.redis.Del(context.Background(), objectStorageScanLockKey).Err() }()
	}

	var untilID string
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		ids, err := p.fileRepo.ListStoredInternalIDs(untilID, objectStorageScanBatchSize)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			break
		}
		for _, id := range ids {
			if err := p.enqueueFile(id); err != nil {
				return err
			}
		}
		untilID = ids[len(ids)-1]
		if len(ids) < objectStorageScanBatchSize {
			break
		}
	}
	slog.Info("object storage migrate scan completed", "lastUntilId", untilID)
	return nil
}

// HandleFile migrates one drive_file row.
func (p *ObjectStorageMigrateProcessor) HandleFile(ctx context.Context, fileID string) error {
	if p.migrator == nil {
		return fmt.Errorf("object storage migrator not configured: %w", driver.SkipRetry)
	}
	if fileID == "" {
		return fmt.Errorf("missing fileId: %w", driver.SkipRetry)
	}
	if err := p.migrator.MigrateFile(ctx, fileID); err != nil {
		if errors.Is(err, coredrive.ErrMigrationNotConfigured) {
			return fmt.Errorf("%w: %w", err, driver.SkipRetry)
		}
		return err
	}
	return nil
}

// Handle dispatches objectStorage migration tasks by type.
func (p *ObjectStorageMigrateProcessor) Handle(ctx context.Context, t driver.Task) error {
	switch t.Type() {
	case queue.TaskTypeObjectStorageMigrateScan:
		return p.HandleScan(ctx)
	case queue.TaskTypeObjectStorageMigrateFile:
		payload, err := queue.DecodeObjectStorageMigrateFilePayload(t.Payload())
		if err != nil {
			return fmt.Errorf("decode migrate file payload: %w: %w", err, driver.SkipRetry)
		}
		return p.HandleFile(ctx, payload.FileID)
	default:
		return fmt.Errorf("unknown task type %q: %w", t.Type(), driver.SkipRetry)
	}
}
