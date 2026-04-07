package wellknown

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/activitypub"
	"github.com/shiroha-a/mk/internal/core/user"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newHandler(t *testing.T) (*Handler, *testutil.MockUserRepository) {
	t.Helper()
	userRepo := testutil.NewMockUserRepository()
	idGen, _ := id.NewGenerator("aidx")
	svc := user.NewService(userRepo, testutil.NewMockNoteRepository(), testutil.NewMockUserNotePiningRepository(), idGen)
	urls := activitypub.NewURLBuilder("https://example.com")
	return NewHandler(urls, svc, "example.com"), userRepo
}

func newReq(t *testing.T, target string) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func addUser(repo *testutil.MockUserRepository, id, username string) {
	repo.Users[id] = &model.User{ID: id, Username: username, UsernameLower: username}
}

func TestWebfinger_AcctWithHost(t *testing.T) {
	h, repo := newHandler(t)
	addUser(repo, "u1", "alice")

	c, rec := newReq(t, "/.well-known/webfinger?resource=acct:alice@example.com")
	require.NoError(t, h.Webfinger(c))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "acct:alice@example.com", resp["subject"])
}

func TestWebfinger_AcctWithoutHost(t *testing.T) {
	h, repo := newHandler(t)
	addUser(repo, "u1", "alice")

	c, rec := newReq(t, "/.well-known/webfinger?resource=acct:alice")
	require.NoError(t, h.Webfinger(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestWebfinger_AcctWrongHost(t *testing.T) {
	h, _ := newHandler(t)
	c, rec := newReq(t, "/.well-known/webfinger?resource=acct:alice@other.example")
	require.NoError(t, h.Webfinger(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestWebfinger_AcctMalformed(t *testing.T) {
	h, _ := newHandler(t)
	c, rec := newReq(t, "/.well-known/webfinger?resource=acct:alice@bob@charlie")
	require.NoError(t, h.Webfinger(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestWebfinger_HTTPSResource(t *testing.T) {
	h, repo := newHandler(t)
	addUser(repo, "u1", "alice")
	c, rec := newReq(t, "/.well-known/webfinger?resource=https://example.com/users/alice")
	require.NoError(t, h.Webfinger(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestWebfinger_HTTPResource(t *testing.T) {
	h, repo := newHandler(t)
	addUser(repo, "u1", "alice")
	c, rec := newReq(t, "/.well-known/webfinger?resource=http://example.com/users/alice")
	require.NoError(t, h.Webfinger(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestWebfinger_HTTPSWrongHost(t *testing.T) {
	h, _ := newHandler(t)
	c, rec := newReq(t, "/.well-known/webfinger?resource=https://other.example/users/alice")
	require.NoError(t, h.Webfinger(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestWebfinger_HTTPSWrongPath(t *testing.T) {
	h, _ := newHandler(t)
	c, rec := newReq(t, "/.well-known/webfinger?resource=https://example.com/something")
	require.NoError(t, h.Webfinger(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestWebfinger_HTTPSInvalidURL(t *testing.T) {
	h, _ := newHandler(t)
	// invalid URL with control char
	c, rec := newReq(t, "/.well-known/webfinger?resource=https://example.com/users/alice%00")
	require.NoError(t, h.Webfinger(c))
	// パースは成功してusersの後ろにごみが付くので NotFound
	assert.NotEqual(t, http.StatusOK, rec.Code)
}

func TestWebfinger_UnknownScheme(t *testing.T) {
	h, _ := newHandler(t)
	c, rec := newReq(t, "/.well-known/webfinger?resource=mailto:alice@example.com")
	require.NoError(t, h.Webfinger(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestWebfinger_NoResource(t *testing.T) {
	h, _ := newHandler(t)
	c, rec := newReq(t, "/.well-known/webfinger")
	require.NoError(t, h.Webfinger(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestWebfinger_UserNotFound(t *testing.T) {
	h, _ := newHandler(t)
	c, rec := newReq(t, "/.well-known/webfinger?resource=acct:ghost@example.com")
	require.NoError(t, h.Webfinger(c))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHostMeta(t *testing.T) {
	h, _ := newHandler(t)
	c, rec := newReq(t, "/.well-known/host-meta")
	require.NoError(t, h.HostMeta(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "webfinger")
}

func TestNodeInfoDiscovery(t *testing.T) {
	h, _ := newHandler(t)
	c, rec := newReq(t, "/.well-known/nodeinfo")
	require.NoError(t, h.NodeInfoDiscovery(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "nodeinfo/2.1")
}
