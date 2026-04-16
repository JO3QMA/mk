package processors_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/queue"
	"github.com/shiroha-a/mk/internal/queue/processors"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func deleteAccountTask(t *testing.T, payload queue.DeleteAccountPayload) *asynq.Task {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	return asynq.NewTask(queue.TaskTypeDeleteAccount, body)
}

func TestDeleteAccountProcessor_DeletesAcrossRepos(t *testing.T) {
	noteRepo := testutil.NewMockNoteRepository()
	driveRepo := testutil.NewMockDriveFileRepository()
	followingRepo := testutil.NewMockFollowingRepository()

	// target ユーザーのコンテンツと、別ユーザーのコンテンツを用意
	noteRepo.Notes["n-target"] = &model.Note{ID: "n-target", UserID: "target"}
	noteRepo.Notes["n-other"] = &model.Note{ID: "n-other", UserID: "other"}
	uid := "target"
	other := "other"
	driveRepo.Files["f-target"] = &model.DriveFile{ID: "f-target", UserID: &uid}
	driveRepo.Files["f-other"] = &model.DriveFile{ID: "f-other", UserID: &other}
	followingRepo.Followings["fo-1"] = &model.Following{ID: "fo-1", FollowerID: "target", FolloweeID: "x"}
	followingRepo.Followings["fo-2"] = &model.Following{ID: "fo-2", FollowerID: "y", FolloweeID: "target"}
	followingRepo.Followings["fo-3"] = &model.Following{ID: "fo-3", FollowerID: "y", FolloweeID: "z"}

	p := processors.NewDeleteAccountProcessor(noteRepo, driveRepo, followingRepo)
	task := deleteAccountTask(t, queue.DeleteAccountPayload{UserID: "target"})
	require.NoError(t, p.Handle(context.Background(), task))

	// target のノートだけ消えている
	assert.NotContains(t, noteRepo.Notes, "n-target")
	assert.Contains(t, noteRepo.Notes, "n-other")

	assert.NotContains(t, driveRepo.Files, "f-target")
	assert.Contains(t, driveRepo.Files, "f-other")

	// target が片方に関与する following 2 件は消え、無関係は残る
	assert.NotContains(t, followingRepo.Followings, "fo-1")
	assert.NotContains(t, followingRepo.Followings, "fo-2")
	assert.Contains(t, followingRepo.Followings, "fo-3")
}

func TestDeleteAccountProcessor_EmptyUserIDSkipsRetry(t *testing.T) {
	p := processors.NewDeleteAccountProcessor(testutil.NewMockNoteRepository(), testutil.NewMockDriveFileRepository(), testutil.NewMockFollowingRepository())
	task := deleteAccountTask(t, queue.DeleteAccountPayload{})
	err := p.Handle(context.Background(), task)
	require.Error(t, err)
	assert.ErrorIs(t, err, asynq.SkipRetry)
}

func TestDeleteAccountProcessor_MalformedPayloadSkipsRetry(t *testing.T) {
	p := processors.NewDeleteAccountProcessor(testutil.NewMockNoteRepository(), testutil.NewMockDriveFileRepository(), testutil.NewMockFollowingRepository())
	task := asynq.NewTask(queue.TaskTypeDeleteAccount, []byte(`not-json`))
	err := p.Handle(context.Background(), task)
	require.Error(t, err)
	assert.ErrorIs(t, err, asynq.SkipRetry)
}

func TestDeleteAccountProcessor_NilReposAreSkipped(t *testing.T) {
	// repo が nil でも panic せず nil error で戻る (部分配線耐性)
	p := processors.NewDeleteAccountProcessor(nil, nil, nil)
	task := deleteAccountTask(t, queue.DeleteAccountPayload{UserID: "target"})
	require.NoError(t, p.Handle(context.Background(), task))
}

// CanceledContext は asynq に retry させるため ctx.Err() を返す (部分実行で
// 成功扱いにしない)。handle が error を返せば MaxRetry 設定が効いて再試行
// されるので孤立した drive_file / following 行が残り続けない。
func TestDeleteAccountProcessor_CanceledContextReturnsError(t *testing.T) {
	noteRepo := testutil.NewMockNoteRepository()
	driveRepo := testutil.NewMockDriveFileRepository()
	followingRepo := testutil.NewMockFollowingRepository()
	uid := "target"
	driveRepo.Files["f"] = &model.DriveFile{ID: "f", UserID: &uid}
	followingRepo.Followings["fo"] = &model.Following{ID: "fo", FollowerID: "target"}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 即キャンセル

	p := processors.NewDeleteAccountProcessor(noteRepo, driveRepo, followingRepo)
	task := deleteAccountTask(t, queue.DeleteAccountPayload{UserID: "target"})
	err := p.Handle(ctx, task)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)

	assert.Contains(t, driveRepo.Files, "f", "ctx canceled なら drive は触らない")
	assert.Contains(t, followingRepo.Followings, "fo", "ctx canceled なら following は触らない")
}

// repo error は上に返る
type failingNoteRepoForDelete struct{ *testutil.MockNoteRepository }

func (f *failingNoteRepoForDelete) DeleteByUserBatch(_ string, _ int) (int64, error) {
	return 0, errors.New("boom")
}

func TestDeleteAccountProcessor_NoteDeleteErrorPropagates(t *testing.T) {
	noteRepo := &failingNoteRepoForDelete{testutil.NewMockNoteRepository()}
	p := processors.NewDeleteAccountProcessor(noteRepo, testutil.NewMockDriveFileRepository(), testutil.NewMockFollowingRepository())
	task := deleteAccountTask(t, queue.DeleteAccountPayload{UserID: "target"})
	err := p.Handle(context.Background(), task)
	require.Error(t, err)
}

// 大量ノートを用意して processor が batch ループで全件処理することを検証。
// ただし単線テストなので pacing sleep を避けるため 120 件 (100+20) にする
// → 第 1 バッチで 100 削除、第 2 バッチで 20 削除 → 合計 120 件、pacing sleep
// は 1 回だけ入る。
type sliceNoteRepo struct {
	*testutil.MockNoteRepository
}

func (s *sliceNoteRepo) DeleteByUserBatch(userID string, batchSize int) (int64, error) {
	return s.MockNoteRepository.DeleteByUserBatch(userID, batchSize)
}

func TestDeleteAccountProcessor_NotesDeletedAcrossMultipleBatches(t *testing.T) {
	noteRepo := testutil.NewMockNoteRepository()
	for i := 0; i < 120; i++ {
		noteRepo.Notes[string(rune('a'+i%26))+"_"+string(rune('0'+i/26))] = &model.Note{
			ID:     "n" + string(rune('A'+i%26)) + string(rune('0'+i/26)),
			UserID: "target",
		}
	}
	// 念のため正確な件数を確認
	require.Len(t, noteRepo.Notes, 120)

	p := processors.NewDeleteAccountProcessor(noteRepo, testutil.NewMockDriveFileRepository(), testutil.NewMockFollowingRepository())
	task := deleteAccountTask(t, queue.DeleteAccountPayload{UserID: "target"})
	require.NoError(t, p.Handle(context.Background(), task))

	// target に紐づくノートはすべて消えている
	for _, n := range noteRepo.Notes {
		assert.NotEqual(t, "target", n.UserID)
	}
}
