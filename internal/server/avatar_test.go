package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shiroha-a/mk/internal/model"
)

// stubAvatarLookup is a hand-rolled minimal mock for the avatar
// handler. Using a custom struct rather than testutil.MockUserRepository
// keeps this test focused on the handler's redirect contract.
type stubAvatarLookup struct {
	users map[string]*model.User // key: "username|host" (host="" for local)
	err   error
}

func (s *stubAvatarLookup) FindByUsernameLower(username string, host *string) (*model.User, error) {
	if s.err != nil {
		return nil, s.err
	}
	key := username + "|"
	if host != nil {
		key += *host
	}
	u, ok := s.users[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return u, nil
}

func newAvatarTestContext(t *testing.T, acct string) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/avatar/@"+acct, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/avatar/@:acct")
	c.SetParamNames("acct")
	c.SetParamValues(acct)
	return c, rec
}

func TestAvatarHandler_LocalUserRedirectsToAvatarURL(t *testing.T) {
	avatarURL := "https://cdn.example/avatars/alice.png"
	repo := &stubAvatarLookup{users: map[string]*model.User{
		"alice|": {ID: "u1", Username: "alice", AvatarURL: &avatarURL},
	}}
	h := avatarHandler(repo, "go.example")

	c, rec := newAvatarTestContext(t, "alice")
	require.NoError(t, h(c))
	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, avatarURL, rec.Header().Get(echo.HeaderLocation))
	assert.Equal(t, "public, max-age=86400", rec.Header().Get(echo.HeaderCacheControl))
}

func TestAvatarHandler_RemoteUserUsesHostFilter(t *testing.T) {
	avatarURL := "https://r/u.png"
	repo := &stubAvatarLookup{users: map[string]*model.User{
		"bob|remote.example": {ID: "u2", Username: "bob", AvatarURL: &avatarURL},
	}}
	h := avatarHandler(repo, "go.example")

	c, rec := newAvatarTestContext(t, "bob@remote.example")
	require.NoError(t, h(c))
	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, avatarURL, rec.Header().Get(echo.HeaderLocation))
}

func TestAvatarHandler_OwnHostTreatedAsLocal(t *testing.T) {
	// acct の host 部が自インスタンス host と一致する場合は local user
	// (host=NULL) として lookup される。
	avatarURL := "https://cdn.example/u3.png"
	repo := &stubAvatarLookup{users: map[string]*model.User{
		"carol|": {ID: "u3", Username: "carol", AvatarURL: &avatarURL},
	}}
	h := avatarHandler(repo, "go.example")

	c, rec := newAvatarTestContext(t, "carol@go.example")
	require.NoError(t, h(c))
	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, avatarURL, rec.Header().Get(echo.HeaderLocation))
}

func TestAvatarHandler_FallsBackToIdenticonWhenNoAvatar(t *testing.T) {
	repo := &stubAvatarLookup{users: map[string]*model.User{
		"dan|": {ID: "u4", Username: "dan"}, // AvatarURL == nil
	}}
	h := avatarHandler(repo, "go.example")

	c, rec := newAvatarTestContext(t, "dan")
	require.NoError(t, h(c))
	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, "/identicon/u4", rec.Header().Get(echo.HeaderLocation))
}

func TestAvatarHandler_UnknownUserRedirectsToStaticFallback(t *testing.T) {
	repo := &stubAvatarLookup{users: map[string]*model.User{}}
	h := avatarHandler(repo, "go.example")

	c, rec := newAvatarTestContext(t, "ghost")
	require.NoError(t, h(c))
	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, avatarStaticFallback, rec.Header().Get(echo.HeaderLocation))
}

func TestAvatarHandler_SuspendedUserHidden(t *testing.T) {
	avatarURL := "https://cdn.example/banned.png"
	repo := &stubAvatarLookup{users: map[string]*model.User{
		"banned|": {ID: "u5", Username: "banned", AvatarURL: &avatarURL, IsSuspended: true},
	}}
	h := avatarHandler(repo, "go.example")

	c, rec := newAvatarTestContext(t, "banned")
	require.NoError(t, h(c))
	assert.Equal(t, http.StatusFound, rec.Code)
	// suspended は user-unknown へ。avatarUrl は露出させない。
	assert.Equal(t, avatarStaticFallback, rec.Header().Get(echo.HeaderLocation))
}

func TestAvatarHandler_RepoErrorFallback(t *testing.T) {
	repo := &stubAvatarLookup{err: errors.New("db down")}
	h := avatarHandler(repo, "go.example")

	c, rec := newAvatarTestContext(t, "alice")
	require.NoError(t, h(c))
	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, avatarStaticFallback, rec.Header().Get(echo.HeaderLocation))
}

func TestAvatarHandler_EmptyAcctFallback(t *testing.T) {
	repo := &stubAvatarLookup{}
	h := avatarHandler(repo, "go.example")

	// Echo router strips the leading `@` via `:acct`、handler は acct ""
	// を受けたら即 fallback する。
	c, rec := newAvatarTestContext(t, "")
	require.NoError(t, h(c))
	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, avatarStaticFallback, rec.Header().Get(echo.HeaderLocation))
}

func TestParseAcct(t *testing.T) {
	cases := []struct {
		input     string
		localHost string
		username  string
		host      *string
	}{
		{"alice", "go.example", "alice", nil},
		{"@alice", "go.example", "alice", nil},
		{"alice@remote.example", "go.example", "alice", strPtr("remote.example")},
		{"alice@GO.EXAMPLE", "go.example", "alice", nil}, // 大文字小文字無視
		{"alice@", "go.example", "alice", nil},
		{"@", "go.example", "", nil},
		{"", "go.example", "", nil},
		{"@bob@cherry.example", "go.example", "bob", strPtr("cherry.example")},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			gotName, gotHost := parseAcct(tc.input, tc.localHost)
			assert.Equal(t, tc.username, gotName)
			if tc.host == nil {
				assert.Nil(t, gotHost)
			} else {
				require.NotNil(t, gotHost)
				assert.Equal(t, *tc.host, *gotHost)
			}
		})
	}
}
