package federation_test

import (
	"errors"
	"testing"

	"github.com/shiroha-a/mk/internal/core/federation"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubFetcher returns canned bytes/error for FetchActor.
type stubFetcher struct {
	body []byte
	err  error
}

func (s *stubFetcher) FetchActor(_ string) ([]byte, error) {
	return s.body, s.err
}

const sampleActor = `{
	"id": "https://remote.example/users/alice",
	"type": "Person",
	"preferredUsername": "alice",
	"name": "Alice",
	"inbox": "https://remote.example/users/alice/inbox",
	"endpoints": {"sharedInbox": "https://remote.example/inbox"},
	"publicKey": {
		"id": "https://remote.example/users/alice#main-key",
		"owner": "https://remote.example/users/alice",
		"publicKeyPem": "-----BEGIN PUBLIC KEY-----\nFAKE\n-----END PUBLIC KEY-----"
	}
}`

func newResolver(t *testing.T, body string, err error) (*federation.Resolver, *testutil.MockUserRepository) {
	t.Helper()
	repo := testutil.NewMockUserRepository()
	idGen, _ := id.NewGenerator("aidx")
	return federation.NewResolver(repo, &stubFetcher{body: []byte(body), err: err}, idGen), repo
}

func TestResolveActor_NewUser(t *testing.T) {
	r, repo := newResolver(t, sampleActor, nil)
	user, err := r.ResolveActor("https://remote.example/users/alice")
	require.NoError(t, err)
	assert.Equal(t, "alice", user.Username)
	assert.Len(t, repo.Users, 1)

	pem, err := r.PublicKeyForActor(user.ID)
	require.NoError(t, err)
	assert.Contains(t, pem, "FAKE")
}

func TestResolveActor_ExistingUser(t *testing.T) {
	r, repo := newResolver(t, sampleActor, nil)
	uri := "https://remote.example/users/alice"
	repo.Users["existing"] = &model.User{
		ID:       "existing",
		Username: "alice",
		URI:      &uri,
	}
	user, err := r.ResolveActor(uri)
	require.NoError(t, err)
	assert.Equal(t, "existing", user.ID)
	// 公開鍵キャッシュは refresh により埋まる
	pem, err := r.PublicKeyForActor("existing")
	require.NoError(t, err)
	assert.Contains(t, pem, "FAKE")
}

func TestResolveActor_FetchError(t *testing.T) {
	r, _ := newResolver(t, "", errors.New("network down"))
	_, err := r.ResolveActor("https://remote.example/users/x")
	assert.Error(t, err)
}

func TestResolveActor_BadJSON(t *testing.T) {
	r, _ := newResolver(t, "{not json", nil)
	_, err := r.ResolveActor("https://remote.example/users/x")
	require.ErrorIs(t, err, federation.ErrInvalidActor)
}

func TestResolveActor_MissingFields(t *testing.T) {
	r, _ := newResolver(t, `{"id":"x"}`, nil)
	_, err := r.ResolveActor("https://remote.example/users/x")
	require.ErrorIs(t, err, federation.ErrInvalidActor)
}

func TestResolveActor_BadHost(t *testing.T) {
	// invalid URL with control char
	body := `{
		"id": "://invalid",
		"preferredUsername": "alice",
		"inbox": "x"
	}`
	r, _ := newResolver(t, body, nil)
	_, err := r.ResolveActor("https://x.example/")
	require.ErrorIs(t, err, federation.ErrInvalidActor)
}

func TestResolveActor_EmptyHost(t *testing.T) {
	// mailto: parses but has no host
	body := `{
		"id": "mailto:alice@example.com",
		"preferredUsername": "alice",
		"inbox": "x"
	}`
	r, _ := newResolver(t, body, nil)
	_, err := r.ResolveActor("https://x.example/")
	require.ErrorIs(t, err, federation.ErrInvalidActor)
}

// failingUserRepo returns Create errors for the resolver.
type failingUserRepo struct {
	*testutil.MockUserRepository
}

func (f *failingUserRepo) Create(_ *model.User) error {
	return errors.New("create failed")
}

func TestResolveActor_RepoCreateError(t *testing.T) {
	mock := testutil.NewMockUserRepository()
	repo := &failingUserRepo{MockUserRepository: mock}
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, &stubFetcher{body: []byte(sampleActor)}, idGen)
	_, err := r.ResolveActor("https://remote.example/users/alice")
	assert.Error(t, err)
}

func TestResolveActorByKeyID(t *testing.T) {
	r, _ := newResolver(t, sampleActor, nil)
	user, err := r.ResolveActorByKeyID("https://remote.example/users/alice#main-key")
	require.NoError(t, err)
	assert.Equal(t, "alice", user.Username)
}

func TestPublicKeyForActor_Missing(t *testing.T) {
	r, _ := newResolver(t, sampleActor, nil)
	_, err := r.PublicKeyForActor("ghost")
	assert.Error(t, err)
}

func TestRefreshPublicKey_OnExistingUser_FetchError(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	uri := "https://remote.example/users/alice"
	repo.Users["existing"] = &model.User{ID: "existing", Username: "alice", URI: &uri}
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, &stubFetcher{err: errors.New("oops")}, idGen)
	user, err := r.ResolveActor(uri)
	require.NoError(t, err)
	assert.Equal(t, "existing", user.ID)
	// fetch失敗してもエラーは返さない
	_, err = r.PublicKeyForActor("existing")
	assert.Error(t, err)
}
