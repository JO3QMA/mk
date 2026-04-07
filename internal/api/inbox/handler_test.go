package inbox

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/activitypub"
	"github.com/shiroha-a/mk/internal/core/federation"
	corefollowing "github.com/shiroha-a/mk/internal/core/following"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubFetcher returns canned actor JSON.
type stubFetcher struct {
	body []byte
}

func (s *stubFetcher) FetchActor(_ string) ([]byte, error) {
	return s.body, nil
}

func actorBody(pubKeyPEM string) string {
	return `{
		"id": "https://remote.example/users/alice",
		"type": "Person",
		"preferredUsername": "alice",
		"name": "Alice",
		"inbox": "https://remote.example/users/alice/inbox",
		"publicKey": {
			"id": "https://remote.example/users/alice#main-key",
			"owner": "https://remote.example/users/alice",
			"publicKeyPem": "` + escapeJSON(pubKeyPEM) + `"
		}
	}`
}

func escapeJSON(s string) string {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		if r == '\n' {
			out = append(out, '\\', 'n')
		} else {
			out = append(out, byte(r))
		}
	}
	return string(out)
}

func newHandler(t *testing.T, pubKeyPEM string) (*Handler, *testutil.MockUserRepository, *testutil.MockFollowingRepository) {
	t.Helper()
	repo := testutil.NewMockUserRepository()
	followingRepo := testutil.NewMockFollowingRepository()
	idGen, _ := id.NewGenerator("aidx")
	resolver := federation.NewResolver(repo, &stubFetcher{body: []byte(actorBody(pubKeyPEM))}, idGen)
	followingSvc := corefollowing.NewService(repo, followingRepo, testutil.NewMockFollowRequestRepository(), idGen)
	processor := federation.NewProcessor(resolver, followingSvc, repo)
	return NewHandler(resolver, processor), repo, followingRepo
}

func newPost(t *testing.T, body []byte) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "https://example.com/inbox", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func TestInbox_HappyPathFollow(t *testing.T) {
	priv, pub, err := activitypub.GenerateRSAKeypair()
	require.NoError(t, err)
	key, err := activitypub.NewPrivateKey("https://remote.example/users/alice#main-key", priv)
	require.NoError(t, err)

	h, repo, followingRepo := newHandler(t, pub)

	// 受信側 bob を登録
	bobURI := "https://example.com/users/bob"
	repo.Users["bob"] = &model.User{ID: "bob", Username: "bob", URI: &bobURI}

	body := []byte(`{"type":"Follow","actor":"https://remote.example/users/alice","object":"https://example.com/users/bob"}`)

	c, rec := newPost(t, body)
	// Echoはリクエストボディをコピーするのでサイン用にreqに直接アクセス
	req := c.Request()
	digest := activitypub.SHA256Digest(body)
	require.NoError(t, activitypub.SignRequest(req, key, digest, []string{"(request-target)", "date", "host", "digest"}))
	// Hostヘッダはサーバ側で復元する必要がある
	req.Host = "example.com"

	require.NoError(t, h.Inbox(c))
	assert.Equal(t, http.StatusAccepted, rec.Code)
	assert.Len(t, followingRepo.Followings, 1)
}

func TestInbox_UnsupportedActivity(t *testing.T) {
	priv, pub, _ := activitypub.GenerateRSAKeypair()
	key, _ := activitypub.NewPrivateKey("https://remote.example/users/alice#main-key", priv)
	h, _, _ := newHandler(t, pub)

	body := []byte(`{"type":"Like","actor":"https://remote.example/users/alice"}`)
	c, rec := newPost(t, body)
	req := c.Request()
	require.NoError(t, activitypub.SignRequest(req, key, activitypub.SHA256Digest(body), []string{"(request-target)", "date", "host", "digest"}))
	req.Host = "example.com"

	require.NoError(t, h.Inbox(c))
	// Unsupportedでも 202 Accepted
	assert.Equal(t, http.StatusAccepted, rec.Code)
}

func TestInbox_BadJSON(t *testing.T) {
	priv, pub, _ := activitypub.GenerateRSAKeypair()
	key, _ := activitypub.NewPrivateKey("https://remote.example/users/alice#main-key", priv)
	h, _, _ := newHandler(t, pub)

	body := []byte(`{not json`)
	c, rec := newPost(t, body)
	req := c.Request()
	require.NoError(t, activitypub.SignRequest(req, key, activitypub.SHA256Digest(body), []string{"(request-target)", "date", "host", "digest"}))
	req.Host = "example.com"

	require.NoError(t, h.Inbox(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestInbox_MissingSignature(t *testing.T) {
	_, pub, _ := activitypub.GenerateRSAKeypair()
	h, _, _ := newHandler(t, pub)

	body := []byte(`{"type":"Follow","actor":"x"}`)
	c, rec := newPost(t, body)
	require.NoError(t, h.Inbox(c))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestInbox_InvalidSignature(t *testing.T) {
	_, pub, _ := activitypub.GenerateRSAKeypair()
	priv2, _, _ := activitypub.GenerateRSAKeypair()
	wrongKey, _ := activitypub.NewPrivateKey("https://remote.example/users/alice#main-key", priv2)

	h, _, _ := newHandler(t, pub)

	body := []byte(`{"type":"Follow","actor":"x"}`)
	c, rec := newPost(t, body)
	req := c.Request()
	require.NoError(t, activitypub.SignRequest(req, wrongKey, activitypub.SHA256Digest(body), []string{"(request-target)", "date", "host", "digest"}))
	req.Host = "example.com"

	require.NoError(t, h.Inbox(c))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestInbox_HostHeaderEmpty(t *testing.T) {
	priv, pub, _ := activitypub.GenerateRSAKeypair()
	key, _ := activitypub.NewPrivateKey("https://remote.example/users/alice#main-key", priv)
	h, _, _ := newHandler(t, pub)

	body := []byte(`{"type":"Like","actor":"https://remote.example/users/alice"}`)
	c, rec := newPost(t, body)
	req := c.Request()
	require.NoError(t, activitypub.SignRequest(req, key, activitypub.SHA256Digest(body), []string{"(request-target)", "date", "host", "digest"}))
	// Host を消す → handler 側で req.Host から復元される
	req.Header.Del("Host")
	req.Host = "example.com"

	require.NoError(t, h.Inbox(c))
	assert.Equal(t, http.StatusAccepted, rec.Code)
}

// readErrReader is an io.Reader that returns an error.
type readErrReader struct{}

func (readErrReader) Read(_ []byte) (int, error) { return 0, assertErrSentinel }

var assertErrSentinel = newSentinelErr("read fail")

type sentinelErr struct{ msg string }

func (e *sentinelErr) Error() string { return e.msg }
func newSentinelErr(s string) error  { return &sentinelErr{msg: s} }

// errorFetcher always errors so the resolver fails.
type errorFetcher struct{}

func (errorFetcher) FetchActor(_ string) ([]byte, error) {
	return nil, assertErrSentinel
}

func TestInbox_ResolverError(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	followingRepo := testutil.NewMockFollowingRepository()
	idGen, _ := id.NewGenerator("aidx")
	resolver := federation.NewResolver(repo, errorFetcher{}, idGen)
	followingSvc := corefollowing.NewService(repo, followingRepo, testutil.NewMockFollowRequestRepository(), idGen)
	processor := federation.NewProcessor(resolver, followingSvc, repo)
	h := NewHandler(resolver, processor)

	priv, _, _ := activitypub.GenerateRSAKeypair()
	key, _ := activitypub.NewPrivateKey("https://remote.example/users/alice#main-key", priv)

	body := []byte(`{"type":"Follow","actor":"x"}`)
	c, rec := newPost(t, body)
	req := c.Request()
	require.NoError(t, activitypub.SignRequest(req, key, activitypub.SHA256Digest(body), []string{"(request-target)", "date", "host", "digest"}))
	req.Host = "example.com"

	require.NoError(t, h.Inbox(c))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestInbox_PublicKeyMissing(t *testing.T) {
	// Pre-populate user with a URI so resolver hits cached path. The fetcher
	// errors out → refreshPublicKey silently fails → keys map stays empty.
	// PublicKeyForActor then returns an error inside verifySignature.
	repo := testutil.NewMockUserRepository()
	uri := "https://remote.example/users/alice"
	repo.Users["alice"] = &model.User{ID: "alice", Username: "alice", URI: &uri}

	followingRepo := testutil.NewMockFollowingRepository()
	idGen, _ := id.NewGenerator("aidx")
	resolver := federation.NewResolver(repo, errorFetcher{}, idGen)
	followingSvc := corefollowing.NewService(repo, followingRepo, testutil.NewMockFollowRequestRepository(), idGen)
	processor := federation.NewProcessor(resolver, followingSvc, repo)
	h := NewHandler(resolver, processor)

	priv, _, _ := activitypub.GenerateRSAKeypair()
	key, _ := activitypub.NewPrivateKey("https://remote.example/users/alice#main-key", priv)

	body := []byte(`{"type":"Follow","actor":"https://remote.example/users/alice"}`)
	c, rec := newPost(t, body)
	req := c.Request()
	require.NoError(t, activitypub.SignRequest(req, key, activitypub.SHA256Digest(body), []string{"(request-target)", "date", "host", "digest"}))
	req.Host = "example.com"

	require.NoError(t, h.Inbox(c))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestInbox_BodyReadError(t *testing.T) {
	_, pub, _ := activitypub.GenerateRSAKeypair()
	h, _, _ := newHandler(t, pub)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "https://example.com/inbox", readErrReader{})
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	require.NoError(t, h.Inbox(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
