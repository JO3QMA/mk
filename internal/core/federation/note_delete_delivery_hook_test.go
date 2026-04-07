package federation_test

import (
	"encoding/json"
	"testing"

	"github.com/shiroha-a/mk/internal/activitypub"
	"github.com/shiroha-a/mk/internal/core/federation"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newDeleteHook(t *testing.T) (
	*federation.NoteDeleteDeliveryHook,
	*stubEnqueuer,
	*testutil.MockUserRepository,
	*testutil.MockFollowingRepository,
	*testutil.MockUserKeypairRepository,
) {
	t.Helper()
	enq := &stubEnqueuer{}
	userRepo := testutil.NewMockUserRepository()
	followingRepo := testutil.NewMockFollowingRepository()
	keypairRepo := testutil.NewMockUserKeypairRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	deliver := federation.NewDeliverService(enq, userRepo, followingRepo, keypairRepo, urls)
	renderer := activitypub.NewRenderer(urls)
	hook := federation.NewNoteDeleteDeliveryHook(deliver, renderer, urls)
	return hook, enq, userRepo, followingRepo, keypairRepo
}

func TestNoteDeleteHook_LocalAuthor(t *testing.T) {
	hook, enq, userRepo, followingRepo, keypairRepo := newDeleteHook(t)
	author := &model.User{ID: "alice", Username: "alice"}
	userRepo.Users["alice"] = author
	keypairRepo.Keypairs["alice"] = &model.UserKeypair{UserID: "alice", PrivateKey: "PEM"}
	followingRepo.RemoteInboxes["alice"] = []string{"https://r.example/inbox"}

	note := &model.Note{ID: "n1", UserID: "alice"}
	hook.OnNoteDeleted(author, note)
	require.Len(t, enq.calls, 1)

	var got map[string]any
	require.NoError(t, json.Unmarshal(enq.calls[0].Body, &got))
	assert.Equal(t, "Delete", got["type"])
	tomb, ok := got["object"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Tombstone", tomb["type"])
	assert.Equal(t, "https://example.com/notes/n1", tomb["id"])
}

func TestNoteDeleteHook_RemoteNoteURI(t *testing.T) {
	// 取り込まれたリモート note を削除する経路 (実際にはほぼ起きないが、URI
	// フォールバック分岐をカバーする)。 author=local、note.URI=remote の混乱した
	// ケースで URI フォールバックが動くか確認。
	hook, enq, userRepo, followingRepo, keypairRepo := newDeleteHook(t)
	author := &model.User{ID: "alice", Username: "alice"}
	userRepo.Users["alice"] = author
	keypairRepo.Keypairs["alice"] = &model.UserKeypair{UserID: "alice", PrivateKey: "PEM"}
	followingRepo.RemoteInboxes["alice"] = []string{"https://r.example/inbox"}

	uri := "https://other.example/notes/x"
	note := &model.Note{ID: "n1", UserID: "alice", URI: &uri}
	hook.OnNoteDeleted(author, note)
	require.Len(t, enq.calls, 1)

	var got map[string]any
	require.NoError(t, json.Unmarshal(enq.calls[0].Body, &got))
	tomb := got["object"].(map[string]any)
	assert.Equal(t, uri, tomb["id"])
}

func TestNoteDeleteHook_RemoteAuthorSkipped(t *testing.T) {
	hook, enq, _, _, _ := newDeleteHook(t)
	host := "remote.example"
	author := &model.User{ID: "bob", Host: &host}
	hook.OnNoteDeleted(author, &model.Note{ID: "n1", UserID: "bob"})
	assert.Empty(t, enq.calls)
}

func TestNoteDeleteHook_LocalOnlySkipped(t *testing.T) {
	hook, enq, userRepo, _, _ := newDeleteHook(t)
	author := &model.User{ID: "alice"}
	userRepo.Users["alice"] = author
	note := &model.Note{ID: "n1", UserID: "alice", LocalOnly: true}
	hook.OnNoteDeleted(author, note)
	assert.Empty(t, enq.calls)
}

func TestNoteDeleteHook_NilArgs(t *testing.T) {
	hook, enq, _, _, _ := newDeleteHook(t)
	hook.OnNoteDeleted(nil, &model.Note{})
	hook.OnNoteDeleted(&model.User{}, nil)
	assert.Empty(t, enq.calls)
}

func TestNoteDeleteHook_DeliverErrorDoesNotPanic(t *testing.T) {
	hook, _, userRepo, followingRepo, _ := newDeleteHook(t)
	author := &model.User{ID: "alice"}
	userRepo.Users["alice"] = author
	// keypair なしで signerCredentials が失敗する
	followingRepo.RemoteInboxes["alice"] = []string{"https://r.example/inbox"}
	note := &model.Note{ID: "n1", UserID: "alice"}
	hook.OnNoteDeleted(author, note)
}
