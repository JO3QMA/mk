package federation_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/activitypub"
	"github.com/shiroha-a/mk/internal/core/federation"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newNoteDeliveryHook(t *testing.T) (
	*federation.NoteDeliveryHook,
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
	idGen, _ := id.NewGenerator("aidx")
	hook := federation.NewNoteDeliveryHook(deliver, renderer, idGen, userRepo)
	return hook, enq, userRepo, followingRepo, keypairRepo
}

func makeLocalAuthor(t *testing.T, userRepo *testutil.MockUserRepository, keypairRepo *testutil.MockUserKeypairRepository) *model.User {
	t.Helper()
	user := &model.User{ID: "alice", Username: "alice"}
	userRepo.Users["alice"] = user
	keypairRepo.Keypairs["alice"] = &model.UserKeypair{
		UserID:     "alice",
		PublicKey:  "PUB",
		PrivateKey: "PEM",
	}
	return user
}

func makeNote(authorID string, visibility model.NoteVisibility) *model.Note {
	idGen, _ := id.NewGenerator("aidx")
	return &model.Note{
		ID:         idGen.Generate(time.Now()),
		UserID:     authorID,
		Visibility: visibility,
	}
}

func TestNoteDeliveryHook_Public_Followers(t *testing.T) {
	hook, enq, userRepo, followingRepo, keypairRepo := newNoteDeliveryHook(t)
	author := makeLocalAuthor(t, userRepo, keypairRepo)
	followingRepo.RemoteInboxes[author.ID] = []string{"https://r.example/inbox"}
	note := makeNote(author.ID, model.NoteVisibilityPublic)

	hook.OnNoteCreated(note, author)
	require.Len(t, enq.calls, 1)
	assert.Equal(t, "https://r.example/inbox", enq.calls[0].Inbox)

	// 配信bodyにCreate activityが入っているか軽く確認
	var got map[string]any
	require.NoError(t, json.Unmarshal(enq.calls[0].Body, &got))
	assert.Equal(t, "Create", got["type"])
}

func TestNoteDeliveryHook_Home_DeliversToFollowers(t *testing.T) {
	hook, enq, userRepo, followingRepo, keypairRepo := newNoteDeliveryHook(t)
	author := makeLocalAuthor(t, userRepo, keypairRepo)
	followingRepo.RemoteInboxes[author.ID] = []string{"https://r.example/inbox"}
	note := makeNote(author.ID, model.NoteVisibilityHome)

	hook.OnNoteCreated(note, author)
	assert.Len(t, enq.calls, 1)
}

func TestNoteDeliveryHook_FollowersOnly_DeliversToFollowers(t *testing.T) {
	hook, enq, userRepo, followingRepo, keypairRepo := newNoteDeliveryHook(t)
	author := makeLocalAuthor(t, userRepo, keypairRepo)
	followingRepo.RemoteInboxes[author.ID] = []string{"https://r.example/inbox"}
	note := makeNote(author.ID, model.NoteVisibilityFollowers)

	hook.OnNoteCreated(note, author)
	assert.Len(t, enq.calls, 1)
}

func TestNoteDeliveryHook_LocalOnly_Skips(t *testing.T) {
	hook, enq, userRepo, _, keypairRepo := newNoteDeliveryHook(t)
	author := makeLocalAuthor(t, userRepo, keypairRepo)
	note := makeNote(author.ID, model.NoteVisibilityPublic)
	note.LocalOnly = true

	hook.OnNoteCreated(note, author)
	assert.Empty(t, enq.calls)
}

func TestNoteDeliveryHook_RemoteAuthor_Skips(t *testing.T) {
	hook, enq, _, _, _ := newNoteDeliveryHook(t)
	host := "remote.example"
	author := &model.User{ID: "bob", Username: "bob", Host: &host}
	note := makeNote(author.ID, model.NoteVisibilityPublic)

	hook.OnNoteCreated(note, author)
	assert.Empty(t, enq.calls)
}

func TestNoteDeliveryHook_NilArgs(t *testing.T) {
	hook, enq, _, _, _ := newNoteDeliveryHook(t)
	// note nil
	hook.OnNoteCreated(nil, &model.User{ID: "x"})
	// author nil
	hook.OnNoteCreated(&model.Note{}, nil)
	assert.Empty(t, enq.calls)
}

func TestNoteDeliveryHook_Specified_RemoteUsers(t *testing.T) {
	hook, enq, userRepo, _, keypairRepo := newNoteDeliveryHook(t)
	author := makeLocalAuthor(t, userRepo, keypairRepo)

	// 配信先: リモートユーザー1人 + ローカル1人 + 不在1人
	host := "remote.example"
	inbox := "https://remote.example/users/r1/inbox"
	userRepo.Users["r1"] = &model.User{ID: "r1", Username: "r1", Host: &host, Inbox: &inbox}
	userRepo.Users["local"] = &model.User{ID: "local", Username: "local"}

	note := makeNote(author.ID, model.NoteVisibilitySpecified)
	note.VisibleUserIDs = []string{"r1", "local", "missing"}

	hook.OnNoteCreated(note, author)
	require.Len(t, enq.calls, 1)
	assert.Equal(t, inbox, enq.calls[0].Inbox)
}

func TestNoteDeliveryHook_Public_DeliverError_DoesNotPanic(t *testing.T) {
	hook, enq, userRepo, followingRepo, _ := newNoteDeliveryHook(t)
	author := &model.User{ID: "alice", Username: "alice"}
	userRepo.Users["alice"] = author
	// keypair が無いので signerCredentials が失敗する → DeliverToFollowers は err
	followingRepo.RemoteInboxes[author.ID] = []string{"https://r.example/inbox"}

	note := makeNote(author.ID, model.NoteVisibilityPublic)
	hook.OnNoteCreated(note, author) // panic しないこと
	assert.Empty(t, enq.calls)
}

func TestNoteDeliveryHook_Specified_DeliverError_DoesNotPanic(t *testing.T) {
	hook, enq, userRepo, _, _ := newNoteDeliveryHook(t)
	author := &model.User{ID: "alice", Username: "alice"}
	userRepo.Users["alice"] = author
	host := "remote.example"
	inbox := "https://remote.example/users/r1/inbox"
	userRepo.Users["r1"] = &model.User{ID: "r1", Username: "r1", Host: &host, Inbox: &inbox}

	note := makeNote(author.ID, model.NoteVisibilitySpecified)
	note.VisibleUserIDs = []string{"r1"}

	hook.OnNoteCreated(note, author)
	assert.Empty(t, enq.calls) // signer key 不在で enqueue 失敗
}
