package federation_test

import (
	"encoding/json"
	"testing"

	"github.com/shiroha-a/mk/internal/activitypub"
	"github.com/shiroha-a/mk/internal/core/federation"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newReactionHook(t *testing.T) (
	*federation.ReactionDeliveryHook,
	*stubEnqueuer,
	*testutil.MockUserRepository,
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
	hook := federation.NewReactionDeliveryHook(deliver, renderer, urls, idGen, userRepo)
	return hook, enq, userRepo, keypairRepo
}

func setupReactor(t *testing.T, userRepo *testutil.MockUserRepository, keypairRepo *testutil.MockUserKeypairRepository) *model.User {
	t.Helper()
	reactor := &model.User{ID: "alice", Username: "alice"}
	userRepo.Users["alice"] = reactor
	keypairRepo.Keypairs["alice"] = &model.UserKeypair{UserID: "alice", PrivateKey: "PEM"}
	return reactor
}

func remoteAuthor(userRepo *testutil.MockUserRepository) (*model.User, *model.Note) {
	host := "remote.example"
	uri := "https://remote.example/users/bob"
	inbox := "https://remote.example/users/bob/inbox"
	bob := &model.User{ID: "bob", Username: "bob", Host: &host, URI: &uri, Inbox: &inbox}
	userRepo.Users["bob"] = bob
	noteURI := "https://remote.example/notes/n1"
	target := &model.Note{ID: "n1", UserID: "bob", URI: &noteURI}
	return bob, target
}

func TestReactionHook_Added_RemoteAuthor(t *testing.T) {
	hook, enq, userRepo, keypairRepo := newReactionHook(t)
	reactor := setupReactor(t, userRepo, keypairRepo)
	_, target := remoteAuthor(userRepo)

	hook.OnReactionAdded(reactor, target, "🎉")
	require.Len(t, enq.calls, 1)
	assert.Equal(t, "https://remote.example/users/bob/inbox", enq.calls[0].Inbox)

	var got map[string]any
	require.NoError(t, json.Unmarshal(enq.calls[0].Body, &got))
	assert.Equal(t, "Like", got["type"])
	assert.Equal(t, "🎉", got["content"])
	assert.Equal(t, "https://remote.example/notes/n1", got["object"])
}

func TestReactionHook_Added_LocalAuthorSkipped(t *testing.T) {
	hook, enq, userRepo, keypairRepo := newReactionHook(t)
	reactor := setupReactor(t, userRepo, keypairRepo)
	bob := &model.User{ID: "bob", Username: "bob"}
	userRepo.Users["bob"] = bob
	target := &model.Note{ID: "n1", UserID: "bob"}

	hook.OnReactionAdded(reactor, target, "🎉")
	assert.Empty(t, enq.calls)
}

func TestReactionHook_Added_RemoteReactorSkipped(t *testing.T) {
	hook, enq, _, _ := newReactionHook(t)
	host := "remote.example"
	reactor := &model.User{ID: "rem", Username: "rem", Host: &host}
	target := &model.Note{ID: "n1", UserID: "bob"}

	hook.OnReactionAdded(reactor, target, "🎉")
	assert.Empty(t, enq.calls)
}

func TestReactionHook_Added_NilArgs(t *testing.T) {
	hook, enq, _, _ := newReactionHook(t)
	hook.OnReactionAdded(nil, &model.Note{}, "x")
	hook.OnReactionAdded(&model.User{}, nil, "x")
	assert.Empty(t, enq.calls)
}

func TestReactionHook_Added_AuthorNotFound(t *testing.T) {
	hook, enq, userRepo, keypairRepo := newReactionHook(t)
	reactor := setupReactor(t, userRepo, keypairRepo)
	target := &model.Note{ID: "n1", UserID: "ghost"}

	hook.OnReactionAdded(reactor, target, "🎉")
	assert.Empty(t, enq.calls)
}

func TestReactionHook_Removed_RemoteAuthor(t *testing.T) {
	hook, enq, userRepo, keypairRepo := newReactionHook(t)
	reactor := setupReactor(t, userRepo, keypairRepo)
	_, target := remoteAuthor(userRepo)

	hook.OnReactionRemoved(reactor, target, "🎉")
	require.Len(t, enq.calls, 1)

	var got map[string]any
	require.NoError(t, json.Unmarshal(enq.calls[0].Body, &got))
	assert.Equal(t, "Undo", got["type"])
	inner, ok := got["object"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Like", inner["type"])
}

func TestReactionHook_Removed_LocalAuthorSkipped(t *testing.T) {
	hook, enq, userRepo, keypairRepo := newReactionHook(t)
	reactor := setupReactor(t, userRepo, keypairRepo)
	userRepo.Users["bob"] = &model.User{ID: "bob", Username: "bob"}
	target := &model.Note{ID: "n1", UserID: "bob"}

	hook.OnReactionRemoved(reactor, target, "🎉")
	assert.Empty(t, enq.calls)
}

func TestReactionHook_LocalNoteFallbackURI(t *testing.T) {
	// target.URI が nil でローカル note の場合は urls.NoteURI() を使う。
	// ただし作者はリモートにしないと配信されないので、targetはリモート → URI 必須。
	// この経路は実際には起きにくいが、buildLike の URI フォールバック分岐をカバー
	// するために URI=nil のリモートnoteで試す。
	hook, enq, userRepo, keypairRepo := newReactionHook(t)
	reactor := setupReactor(t, userRepo, keypairRepo)
	host := "remote.example"
	uri := "https://remote.example/users/bob"
	inbox := "https://remote.example/users/bob/inbox"
	userRepo.Users["bob"] = &model.User{ID: "bob", Username: "bob", Host: &host, URI: &uri, Inbox: &inbox}
	target := &model.Note{ID: "n1", UserID: "bob"} // URI=nil

	hook.OnReactionAdded(reactor, target, "🎉")
	require.Len(t, enq.calls, 1)
	var got map[string]any
	require.NoError(t, json.Unmarshal(enq.calls[0].Body, &got))
	assert.Equal(t, "https://example.com/notes/n1", got["object"])
}

func TestReactionHook_DeliverErrorDoesNotPanic(t *testing.T) {
	hook, _, userRepo, _ := newReactionHook(t)
	// keypair なしで signerCredentials が失敗 → DeliverToUser が err
	reactor := &model.User{ID: "alice", Username: "alice"}
	userRepo.Users["alice"] = reactor
	_, target := remoteAuthor(userRepo)

	hook.OnReactionAdded(reactor, target, "🎉")
	hook.OnReactionRemoved(reactor, target, "🎉")
}
