package federation_test

import (
	"testing"

	"github.com/shiroha-a/mk/internal/activitypub"
	"github.com/shiroha-a/mk/internal/core/federation"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
)

func TestUserMentionResolver_Local(t *testing.T) {
	userRepo := testutil.NewMockUserRepository()
	userRepo.Users["uA"] = &model.User{ID: "uA", Username: "alice", UsernameLower: "alice"}
	urls := activitypub.NewURLBuilder("https://example.com")
	r := federation.NewUserMentionResolver(userRepo, urls)

	name, uri, ok := r.ResolveMention("uA")
	assert.True(t, ok)
	assert.Equal(t, "@alice", name)
	assert.Equal(t, "https://example.com/users/uA", uri)
}

func TestUserMentionResolver_Remote(t *testing.T) {
	userRepo := testutil.NewMockUserRepository()
	host := "remote.example"
	remoteURI := "https://remote.example/users/bob"
	userRepo.Users["uB"] = &model.User{
		ID:            "uB",
		Username:      "bob",
		UsernameLower: "bob",
		Host:          &host,
		URI:           &remoteURI,
	}
	urls := activitypub.NewURLBuilder("https://example.com")
	r := federation.NewUserMentionResolver(userRepo, urls)

	name, uri, ok := r.ResolveMention("uB")
	assert.True(t, ok)
	assert.Equal(t, "@bob@remote.example", name)
	assert.Equal(t, remoteURI, uri)
}

func TestUserMentionResolver_NotFound(t *testing.T) {
	userRepo := testutil.NewMockUserRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	r := federation.NewUserMentionResolver(userRepo, urls)

	_, _, ok := r.ResolveMention("ghost")
	assert.False(t, ok)
}

func TestUserMentionResolver_RemoteNoURI(t *testing.T) {
	userRepo := testutil.NewMockUserRepository()
	host := "remote.example"
	userRepo.Users["uB"] = &model.User{
		ID:       "uB",
		Username: "bob",
		Host:     &host,
	}
	urls := activitypub.NewURLBuilder("https://example.com")
	r := federation.NewUserMentionResolver(userRepo, urls)

	_, _, ok := r.ResolveMention("uB")
	assert.False(t, ok)
}
