package chat_test

import (
	"context"
	"testing"

	"github.com/shiroha-a/mk/internal/activitypub"
	corechat "github.com/shiroha-a/mk/internal/core/chat"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAPDeliverer struct {
	called    int
	lastBody  []byte
	returnErr error
}

func (f *fakeAPDeliverer) DeliverToUser(_ string, _ *model.User, body []byte) error {
	f.called++
	f.lastBody = body
	return f.returnErr
}

func newAPService(t *testing.T) (*corechat.Service, *testutil.MockUserRepository, *fakeAPDeliverer) {
	t.Helper()
	chatRepo := newFakeRepo()
	idGen, _ := id.NewGenerator("aidx")
	svc := corechat.NewService(chatRepo, idGen)
	userRepo := testutil.NewMockUserRepository()
	urls := activitypub.NewURLBuilder("https://local.example")
	renderer := activitypub.NewRenderer(urls)
	deliverer := &fakeAPDeliverer{}
	svc.SetAPDelivery(userRepo, renderer, urls, deliverer)
	return svc, userRepo, deliverer
}

func TestCreateMessageToUser_DeliversToRemoteUser(t *testing.T) {
	svc, userRepo, deliverer := newAPService(t)
	remoteHost := "remote.example"
	remoteURI := "https://remote.example/users/bob"
	userRepo.Users["alice"] = &model.User{ID: "alice", Username: "alice"}
	userRepo.Users["bob"] = &model.User{ID: "bob", Username: "bob", Host: &remoteHost, URI: &remoteURI}

	msg, err := svc.CreateMessageToUser(context.Background(), "alice", "bob", "hello remote", "")
	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Equal(t, 1, deliverer.called)
	assert.Contains(t, string(deliverer.lastBody), "Misskey:ChatMessage")
	assert.Contains(t, string(deliverer.lastBody), "hello remote")
}

func TestCreateMessageToUser_SkipsLocalRecipient(t *testing.T) {
	svc, userRepo, deliverer := newAPService(t)
	userRepo.Users["alice"] = &model.User{ID: "alice", Username: "alice"}
	userRepo.Users["bob"] = &model.User{ID: "bob", Username: "bob"}

	_, err := svc.CreateMessageToUser(context.Background(), "alice", "bob", "local msg", "")
	require.NoError(t, err)
	assert.Equal(t, 0, deliverer.called)
}

func TestCreateMessageToUser_DeliveryFailureIsSwallowed(t *testing.T) {
	svc, userRepo, deliverer := newAPService(t)
	deliverer.returnErr = assert.AnError
	remoteHost := "remote.example"
	remoteURI := "https://remote.example/users/bob"
	userRepo.Users["alice"] = &model.User{ID: "alice", Username: "alice"}
	userRepo.Users["bob"] = &model.User{ID: "bob", Username: "bob", Host: &remoteHost, URI: &remoteURI}

	msg, err := svc.CreateMessageToUser(context.Background(), "alice", "bob", "hi", "")
	require.NoError(t, err, "delivery failure must not propagate")
	require.NotNil(t, msg)
	assert.Equal(t, 1, deliverer.called)
}

func TestCreateMessageViaAP(t *testing.T) {
	chatRepo := newFakeRepo()
	idGen, _ := id.NewGenerator("aidx")
	svc := corechat.NewService(chatRepo, idGen)
	sender := &model.User{ID: "remote1", Username: "remote1"}

	msg, err := svc.CreateMessageViaAP(context.Background(), "https://remote.example/chat-messages/1", sender, "local1", "hello from remote")
	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Equal(t, "remote1", msg.FromUserID)
	require.NotNil(t, msg.ToUserID)
	assert.Equal(t, "local1", *msg.ToUserID)
	require.NotNil(t, msg.URI)
	assert.Equal(t, "https://remote.example/chat-messages/1", *msg.URI)
}

func TestCreateMessageViaAP_InvalidTarget(t *testing.T) {
	chatRepo := newFakeRepo()
	idGen, _ := id.NewGenerator("aidx")
	svc := corechat.NewService(chatRepo, idGen)

	_, err := svc.CreateMessageViaAP(context.Background(), "", nil, "local1", "hi")
	assert.Error(t, err)

	_, err = svc.CreateMessageViaAP(context.Background(), "", &model.User{ID: "x"}, "", "hi")
	assert.Error(t, err)
}
