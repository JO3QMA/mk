package processors

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"
	"github.com/shiroha-a/mk/internal/queue"
	"github.com/shiroha-a/mk/internal/repository"
)

// DeleteAccountProcessor cascades through a user's related rows (notes,
// drive files, follow graph) after admin/accounts/delete has flipped the
// soft-delete flags. User の行そのものは消さず (username/profile を
// moderation log のために保持する) isDeleted=true のまま残す。
type DeleteAccountProcessor struct {
	noteRepo      repository.NoteRepository
	driveFileRepo repository.DriveFileRepository
	followingRepo repository.FollowingRepository
}

// NewDeleteAccountProcessor wires the processor with the repositories it
// needs. 必要な repo が nil ならその種別の削除はスキップする (テストや
// 部分起動で邪魔しないようにする)。
func NewDeleteAccountProcessor(noteRepo repository.NoteRepository, driveFileRepo repository.DriveFileRepository, followingRepo repository.FollowingRepository) *DeleteAccountProcessor {
	return &DeleteAccountProcessor{
		noteRepo:      noteRepo,
		driveFileRepo: driveFileRepo,
		followingRepo: followingRepo,
	}
}

// Handle implements asynq.Handler.
func (p *DeleteAccountProcessor) Handle(ctx context.Context, t *asynq.Task) error {
	payload, err := queue.DecodeDeleteAccountPayload(t.Payload())
	if err != nil {
		return fmt.Errorf("decode delete-account payload: %w: %w", err, asynq.SkipRetry)
	}
	if payload.UserID == "" {
		return fmt.Errorf("delete-account: userId is required: %w", asynq.SkipRetry)
	}

	// Notes: 100 件/バッチで順次 delete。途中で ctx.Done() なら諦めて
	// retry に任せる。
	const noteBatchSize = 100
	if p.noteRepo != nil {
		if ctx.Err() == nil {
			deleted, err := p.noteRepo.DeleteByUser(payload.UserID, noteBatchSize)
			if err != nil {
				slog.Error("delete-account: note purge failed",
					"userId", payload.UserID, "err", err)
				return err
			}
			if deleted > 0 {
				slog.Info("delete-account: notes deleted",
					"userId", payload.UserID, "count", deleted)
			}
		}
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
