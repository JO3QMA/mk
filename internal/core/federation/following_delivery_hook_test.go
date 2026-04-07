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

func newFollowingHook(t *testing.T) (
	*federation.FollowingDeliveryHook,
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
	hook := federation.NewFollowingDeliveryHook(deliver, renderer, urls)
	return hook, enq, userRepo, keypairRepo
}

func makeRemote(id, host, uri, inbox string) *model.User {
	h, u, i := host, uri, inbox
	return &model.User{ID: id, Username: id, Host: &h, URI: &u, Inbox: &i}
}

func makeLocal(id string) *model.User {
	return &model.User{ID: id, Username: id}
}

func TestFollowingHook_OnLocalFollowed_Remote(t *testing.T) {
	hook, enq, userRepo, keypairRepo := newFollowingHook(t)
	follower := makeLocal("alice")
	userRepo.Users["alice"] = follower
	keypairRepo.Keypairs["alice"] = &model.UserKeypair{UserID: "alice", PrivateKey: "PEM"}
	followee := makeRemote("bob", "remote.example", "https://remote.example/users/bob", "https://remote.example/users/bob/inbox")

	hook.OnLocalFollowed(follower, followee)
	require.Len(t, enq.calls, 1)
	assert.Equal(t, "https://remote.example/users/bob/inbox", enq.calls[0].Inbox)

	var got map[string]any
	require.NoError(t, json.Unmarshal(enq.calls[0].Body, &got))
	assert.Equal(t, "Follow", got["type"])
}

func TestFollowingHook_OnLocalFollowed_LocalFolloweeSkipped(t *testing.T) {
	hook, enq, _, _ := newFollowingHook(t)
	hook.OnLocalFollowed(makeLocal("alice"), makeLocal("bob"))
	assert.Empty(t, enq.calls)
}

func TestFollowingHook_OnLocalFollowed_RemoteFollowerSkipped(t *testing.T) {
	hook, enq, _, _ := newFollowingHook(t)
	rem := makeRemote("alice", "h", "https://h/u", "https://h/u/inbox")
	rem2 := makeRemote("bob", "h", "https://h/u2", "https://h/u2/inbox")
	hook.OnLocalFollowed(rem, rem2)
	assert.Empty(t, enq.calls)
}

func TestFollowingHook_OnLocalFollowed_NilArgs(t *testing.T) {
	hook, enq, _, _ := newFollowingHook(t)
	hook.OnLocalFollowed(nil, makeLocal("bob"))
	hook.OnLocalFollowed(makeLocal("alice"), nil)
	assert.Empty(t, enq.calls)
}

func TestFollowingHook_OnLocalFollowed_RemoteWithoutURI(t *testing.T) {
	hook, enq, _, _ := newFollowingHook(t)
	host := "remote.example"
	bad := &model.User{ID: "bob", Username: "bob", Host: &host}
	hook.OnLocalFollowed(makeLocal("alice"), bad)
	assert.Empty(t, enq.calls)
}

func TestFollowingHook_OnLocalUnfollowed_Remote(t *testing.T) {
	hook, enq, userRepo, keypairRepo := newFollowingHook(t)
	follower := makeLocal("alice")
	userRepo.Users["alice"] = follower
	keypairRepo.Keypairs["alice"] = &model.UserKeypair{UserID: "alice", PrivateKey: "PEM"}
	followee := makeRemote("bob", "remote.example", "https://remote.example/users/bob", "https://remote.example/users/bob/inbox")

	hook.OnLocalUnfollowed(follower, followee)
	require.Len(t, enq.calls, 1)

	var got map[string]any
	require.NoError(t, json.Unmarshal(enq.calls[0].Body, &got))
	assert.Equal(t, "Undo", got["type"])
	inner, ok := got["object"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Follow", inner["type"])
}

func TestFollowingHook_OnLocalUnfollowed_LocalSkipped(t *testing.T) {
	hook, enq, _, _ := newFollowingHook(t)
	hook.OnLocalUnfollowed(makeLocal("a"), makeLocal("b"))
	assert.Empty(t, enq.calls)
}

func TestFollowingHook_OnLocalFollowAccepted_Remote(t *testing.T) {
	hook, enq, userRepo, keypairRepo := newFollowingHook(t)
	// follower がリモート、followee (=accept する側) がローカル
	follower := makeRemote("bob", "remote.example", "https://remote.example/users/bob", "https://remote.example/users/bob/inbox")
	followee := makeLocal("alice")
	userRepo.Users["alice"] = followee
	keypairRepo.Keypairs["alice"] = &model.UserKeypair{UserID: "alice", PrivateKey: "PEM"}

	hook.OnLocalFollowAccepted(follower, followee)
	require.Len(t, enq.calls, 1)

	var got map[string]any
	require.NoError(t, json.Unmarshal(enq.calls[0].Body, &got))
	assert.Equal(t, "Accept", got["type"])
	inner, ok := got["object"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Follow", inner["type"])
	assert.Equal(t, *follower.URI, inner["actor"])
}

func TestFollowingHook_OnLocalFollowAccepted_NilArgs(t *testing.T) {
	hook, enq, _, _ := newFollowingHook(t)
	hook.OnLocalFollowAccepted(nil, makeLocal("a"))
	hook.OnLocalFollowAccepted(makeLocal("a"), nil)
	assert.Empty(t, enq.calls)
}

func TestFollowingHook_OnLocalFollowAccepted_BothLocalSkipped(t *testing.T) {
	hook, enq, _, _ := newFollowingHook(t)
	hook.OnLocalFollowAccepted(makeLocal("a"), makeLocal("b"))
	assert.Empty(t, enq.calls)
}

func TestFollowingHook_OnLocalFollowAccepted_RemoteFolloweeSkipped(t *testing.T) {
	hook, enq, _, _ := newFollowingHook(t)
	rem1 := makeRemote("a", "h", "https://h/a", "https://h/a/inbox")
	rem2 := makeRemote("b", "h", "https://h/b", "https://h/b/inbox")
	hook.OnLocalFollowAccepted(rem1, rem2)
	assert.Empty(t, enq.calls)
}

func TestFollowingHook_OnLocalFollowAccepted_RemoteFollowerWithoutURI(t *testing.T) {
	hook, enq, _, _ := newFollowingHook(t)
	host := "h"
	rem := &model.User{ID: "bob", Username: "bob", Host: &host}
	hook.OnLocalFollowAccepted(rem, makeLocal("alice"))
	assert.Empty(t, enq.calls)
}

func TestFollowingHook_DeliverErrors_DoNotPanic(t *testing.T) {
	hook, _, userRepo, _ := newFollowingHook(t)
	// keypair 不在で signerCredentials が失敗する → DeliverToUser は err
	follower := makeLocal("alice")
	userRepo.Users["alice"] = follower
	followee := makeRemote("bob", "remote.example", "https://remote.example/users/bob", "https://remote.example/users/bob/inbox")
	hook.OnLocalFollowed(follower, followee)
	hook.OnLocalUnfollowed(follower, followee)
	hook.OnLocalFollowAccepted(followee, follower) // ← follower=remote, followee=local だが key不在
}
