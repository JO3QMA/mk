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

// ObjectStorageScanLockKey prevents concurrent migrateScan workers (#1476).
const ObjectStorageScanLockKey = "mk:drive:objectStorage:migrateScan:lock"

// ObjectStorageScanLockTTL bounds how long a crashed scan worker blocks rescheduling.
const ObjectStorageScanLockTTL = 6 * time.Hour

// ObjectStorageMigrateProcessor handles local→S3 drive migration tasks (#1476).
type ObjectStorageMigrateProcessor struct {
	migrator      *coredrive.Migrator
	fileRepo      repository.DriveFileRepository
	enqueueFile   func(fileID string) error
	enqueueScan   func(untilID string) error
	scanBatchSize int
	redis         redis.UniversalClient
}

// ObjectStorageMigrateProcessorConfig wires dependencies for the processor.
type ObjectStorageMigrateProcessorConfig struct {
	Migrator      *coredrive.Migrator
	FileRepo      repository.DriveFileRepository
	EnqueueFile   func(fileID string) error
	EnqueueScan   func(untilID string) error
	ScanBatchSize int
	Redis         redis.UniversalClient
}

// NewObjectStorageMigrateProcessor constructs the processor.
func NewObjectStorageMigrateProcessor(cfg ObjectStorageMigrateProcessorConfig) *ObjectStorageMigrateProcessor {
	batch := cfg.ScanBatchSize
	if batch <= 0 {
		batch = 100
	}
	return &ObjectStorageMigrateProcessor{
		migrator:      cfg.Migrator,
		fileRepo:      cfg.FileRepo,
		enqueueFile:   cfg.EnqueueFile,
		enqueueScan:   cfg.EnqueueScan,
		scanBatchSize: batch,
		redis:         cfg.Redis,
	}
}

// HandleScan enqueues migrateFile jobs for one batch of storedInternal rows and
// chains another scan task when more rows remain (#1476).
func (p *ObjectStorageMigrateProcessor) HandleScan(ctx context.Context, untilID string) error {
	if p.fileRepo == nil || p.enqueueFile == nil {
		return fmt.Errorf("object storage migrate scan not configured: %w", driver.SkipRetry)
	}
	if p.redis != nil {
		ok, err := p.redis.SetNX(ctx, ObjectStorageScanLockKey, "1", ObjectStorageScanLockTTL).Result()
		if err != nil {
			return err
		}
		if !ok {
			slog.Info("object storage migrate scan skipped: lock held")
			return nil
		}
	}

	releaseLock := func() {
		if p.redis != nil {
			_ = p.redis.Del(context.Background(), ObjectStorageScanLockKey).Err()
		}
	}

	ids, err := p.fileRepo.ListStoredInternalIDs(untilID, p.scanBatchSize)
	if err != nil {
		releaseLock()
		return err
	}
	for _, id := range ids {
		if ctx.Err() != nil {
			releaseLock()
			return ctx.Err()
		}
		if err := p.enqueueFile(id); err != nil {
			releaseLock()
			return err
		}
	}

	hasMore := len(ids) >= p.scanBatchSize
	releaseLock()

	if hasMore {
		if p.enqueueScan == nil {
			return fmt.Errorf("object storage migrate scan chain not configured: %w", driver.SkipRetry)
		}
		nextUntil := ids[len(ids)-1]
		if err := p.enqueueScan(nextUntil); err != nil {
			return err
		}
		slog.Info("object storage migrate scan chained next batch", "batchSize", len(ids), "nextUntilId", nextUntil)
		return nil
	}
	slog.Info("object storage migrate scan completed", "lastBatch", len(ids))
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
		scanPayload, err := queue.DecodeObjectStorageMigrateScanPayload(t.Payload())
		if err != nil {
			return fmt.Errorf("decode migrate scan payload: %w: %w", err, driver.SkipRetry)
		}
		return p.HandleScan(ctx, scanPayload.UntilID)
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
