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

// CanceledContext で途中から先がスキップされる挙動の確認。ここでは
// ctx.Cancel() を手動で行い、最初の phase (notes) だけ実行 → drive/following
// はスキップされることを期待する。ただし並行実行でなく単線なので、
// note 削除自体は一度走る点に注意。
func TestDeleteAccountProcessor_CanceledContextStopsEarly(t *testing.T) {
	noteRepo := testutil.NewMockNoteRepository()
	driveRepo := testutil.NewMockDriveFileRepository()
	followingRepo := testutil.NewMockFollowingRepository()
	uid := "target"
	driveRepo.Files["f"] = &model.DriveFile{ID: "f", UserID: &uid}
	followingRepo.Followings["fo"] = &model.Following{ID: "fo", FollowerID: "target"}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 即キャンセル → すべての phase がスキップされる

	p := processors.NewDeleteAccountProcessor(noteRepo, driveRepo, followingRepo)
	task := deleteAccountTask(t, queue.DeleteAccountPayload{UserID: "target"})
	require.NoError(t, p.Handle(ctx, task))

	assert.Contains(t, driveRepo.Files, "f", "ctx canceled なら drive は触らない")
	assert.Contains(t, followingRepo.Followings, "fo", "ctx canceled なら following は触らない")
}

// repo error は上に返る
type failingNoteRepoForDelete struct{ *testutil.MockNoteRepository }

func (f *failingNoteRepoForDelete) DeleteByUser(_ string, _ int) (int64, error) {
	return 0, errors.New("boom")
}

func TestDeleteAccountProcessor_NoteDeleteErrorPropagates(t *testing.T) {
	noteRepo := &failingNoteRepoForDelete{testutil.NewMockNoteRepository()}
	p := processors.NewDeleteAccountProcessor(noteRepo, testutil.NewMockDriveFileRepository(), testutil.NewMockFollowingRepository())
	task := deleteAccountTask(t, queue.DeleteAccountPayload{UserID: "target"})
	err := p.Handle(context.Background(), task)
	require.Error(t, err)
}
