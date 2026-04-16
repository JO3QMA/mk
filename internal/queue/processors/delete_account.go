package processors

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
	"github.com/shiroha-a/mk/internal/queue"
	"github.com/shiroha-a/mk/internal/repository"
)

// DeleteAccountProcessor cascades through a user's related rows (notes,
// drive files, follow graph) after admin/accounts/delete has flipped the
// soft-delete flags. The user row itself is NOT deleted — it is kept for
// moderation log retention with only isDeleted=true set.
type DeleteAccountProcessor struct {
	noteRepo      repository.NoteRepository
	driveFileRepo repository.DriveFileRepository
	followingRepo repository.FollowingRepository
}

// NewDeleteAccountProcessor wires the processor with the repositories it
// needs. Any nil repository causes that deletion phase to be skipped, so
// tests and partial-start configurations do not need to supply the full
// repository set.
func NewDeleteAccountProcessor(noteRepo repository.NoteRepository, driveFileRepo repository.DriveFileRepository, followingRepo repository.FollowingRepository) *DeleteAccountProcessor {
	return &DeleteAccountProcessor{
		noteRepo:      noteRepo,
		driveFileRepo: driveFileRepo,
		followingRepo: followingRepo,
	}
}

// deleteAccountNoteBatchSize controls how many notes are deleted per loop
// iteration. Kept small enough that one batch finishes well within asynq's
// default per-task timeout and the processor can check ctx.Err() between
// batches.
const deleteAccountNoteBatchSize = 100

// deleteAccountBatchSleep throttles DB I/O between note batches to avoid
// saturating the write path for accounts with very large note histories.
const deleteAccountBatchSleep = 250 * time.Millisecond

// Handle implements asynq.Handler.
func (p *DeleteAccountProcessor) Handle(ctx context.Context, t *asynq.Task) error {
	payload, err := queue.DecodeDeleteAccountPayload(t.Payload())
	if err != nil {
		return fmt.Errorf("decode delete-account payload: %w: %w", err, asynq.SkipRetry)
	}
	if payload.UserID == "" {
		return fmt.Errorf("delete-account: userId is required: %w", asynq.SkipRetry)
	}

	if err := p.deleteNotes(ctx, payload.UserID); err != nil {
		return err
	}

	if p.driveFileRepo != nil && ctx.Err() == nil {
		deleted, err := p.driveFileRepo.DeleteByUser(payload.UserID)
		if err != nil {
			slog.Error("delete-account: drive purge failed",
				"userId", payload.UserID, "err", err)
			return err
		}
		if deleted > 0 {
			slog.Info("delete-account: drive files deleted",
				"userId", payload.UserID, "count", deleted)
		}
	}

	if p.followingRepo != nil && ctx.Err() == nil {
		deleted, err := p.followingRepo.DeleteAllByUser(payload.UserID)
		if err != nil {
			slog.Error("delete-account: following purge failed",
				"userId", payload.UserID, "err", err)
			return err
		}
		if deleted > 0 {
			slog.Info("delete-account: following rows deleted",
				"userId", payload.UserID, "count", deleted)
		}
	}
	return nil
}

// deleteNotes loops DeleteByUserBatch so that cancellation checkpoints and
// throttling live here instead of in the repository. 大量ノート保有ユーザー
// (10万件規模) に備えて各 batch の後で ctx.Err() を見て中断する。
func (p *DeleteAccountProcessor) deleteNotes(ctx context.Context, userID string) error {
	if p.noteRepo == nil {
		return nil
	}
	var total int64
	for {
		if ctx.Err() != nil {
			slog.Warn("delete-account: note purge interrupted",
				"userId", userID, "deleted", total, "err", ctx.Err())
			return nil
		}
		deleted, err := p.noteRepo.DeleteByUserBatch(userID, deleteAccountNoteBatchSize)
		if err != nil {
			slog.Error("delete-account: note purge failed",
				"userId", userID, "err", err)
			return err
		}
		total += deleted
		if deleted < int64(deleteAccountNoteBatchSize) {
			break
		}
		// I/O 平準化の短い sleep。ctx が生きている間のみ効かせる。
		select {
		case <-ctx.Done():
			slog.Warn("delete-account: note purge canceled during pacing",
				"userId", userID, "deleted", total)
			return nil
		case <-time.After(deleteAccountBatchSleep):
		}
	}
	if total > 0 {
		slog.Info("delete-account: notes deleted",
			"userId", userID, "count", total)
	}
	return nil
}
