package ap

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/activitypub"
	corenote "github.com/shiroha-a/mk/internal/core/note"
	coreuser "github.com/shiroha-a/mk/internal/core/user"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memoryKeypairRepo is an in-memory repository for tests.
type memoryKeypairRepo struct {
	items map[string]*model.UserKeypair
	err   error
}

func (m *memoryKeypairRepo) Create(k *model.UserKeypair) error {
	if m.err != nil {
		return m.err
	}
	m.items[k.UserID] = k
	return nil
}

func (m *memoryKeypairRepo) FindByUserID(userID string) (*model.UserKeypair, error) {
	if m.err != nil {
		return nil, m.err
	}
	k, ok := m.items[userID]
	if !ok {
		return nil, errors.New("not found")
	}
	return k, nil
}

var _ repository.UserKeypairRepository = (*memoryKeypairRepo)(nil)

func newHandler(t *testing.T) (*Handler, *testutil.MockUserRepository, *testutil.MockNoteRepository, *memoryKeypairRepo) {
	t.Helper()
	userRepo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	piningRepo := testutil.NewMockUserNotePiningRepository()
	idGen, _ := id.NewGenerator("aidx")
	userSvc := coreuser.NewService(userRepo, noteRepo, piningRepo, idGen)
	querySvc := corenote.NewQueryService(noteRepo, nil)
	keypairRepo := &memoryKeypairRepo{items: map[string]*model.UserKeypair{}}
	urls := activitypub.NewURLBuilder("https://example.com")
	renderer := activitypub.NewRenderer(urls)
	return NewHandler(renderer, userSvc, querySvc, keypairRepo, idGen), userRepo, noteRepo, keypairRepo
}

func newReq(t *testing.T, paramName, paramValue string) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames(paramName)
	c.SetParamValues(paramValue)
	return c, rec
}

func TestUser_Success(t *testing.T) {
	h, userRepo, _, keypairRepo := newHandler(t)
	userRepo.Users["u1"] = &model.User{ID: "u1", Username: "alice"}
	keypairRepo.items["u1"] = &model.UserKeypair{UserID: "u1", PublicKey: "PUBKEY"}

	c, rec := newReq(t, "id", "u1")
	require.NoError(t, h.User(c))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Person")
	assert.Contains(t, rec.Body.String(), "alice")
}

func TestUser_NotFound(t *testing.T) {
	h, _, _, _ := newHandler(t)
	c, rec := newReq(t, "id", "ghost")
	require.NoError(t, h.User(c))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUser_Remote(t *testing.T) {
	h, userRepo, _, _ := newHandler(t)
	host := "remote.example"
	userRepo.Users["u1"] = &model.User{ID: "u1", Username: "alice", Host: &host}
	c, rec := newReq(t, "id", "u1")
	require.NoError(t, h.User(c))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUser_KeypairFetchError(t *testing.T) {
	h, userRepo, _, keypairRepo := newHandler(t)
	userRepo.Users["u1"] = &model.User{ID: "u1", Username: "alice"}
	keypairRepo.err = errors.New("db down")
	c, rec := newReq(t, "id", "u1")
	require.NoError(t, h.User(c))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestNote_Success(t *testing.T) {
	h, _, noteRepo, _ := newHandler(t)
	noteRepo.Notes["n1"] = &model.Note{
		ID: "n1", UserID: "u1", Visibility: model.NoteVisibilityPublic,
	}
	c, rec := newReq(t, "id", "n1")
	require.NoError(t, h.Note(c))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Note")
}

func TestNote_NotFound(t *testing.T) {
	h, _, _, _ := newHandler(t)
	c, rec := newReq(t, "id", "ghost")
	require.NoError(t, h.Note(c))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestNote_Remote(t *testing.T) {
	h, _, noteRepo, _ := newHandler(t)
	host := "remote.example"
	noteRepo.Notes["n1"] = &model.Note{
		ID: "n1", UserID: "u1", Visibility: model.NoteVisibilityPublic, UserHost: &host,
	}
	c, rec := newReq(t, "id", "n1")
	require.NoError(t, h.Note(c))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// --- API endpoint tests ---

func postJSON(handler func(echo.Context) error, body string) *httptest.ResponseRecorder {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	_ = handler(c)
	return rec
}

func TestAPIGet_Success_Note(t *testing.T) {
	h, _, noteRepo, _ := newHandler(t)
	noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "u1", Visibility: model.NoteVisibilityPublic}
	rec := postJSON(h.APIGet, `{"uri":"https://example.com/notes/n1"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Note")
}

func TestAPIGet_Success_User(t *testing.T) {
	h, userRepo, _, keypairRepo := newHandler(t)
	userRepo.Users["u1"] = &model.User{ID: "u1", Username: "alice"}
	keypairRepo.items["u1"] = &model.UserKeypair{UserID: "u1", PublicKey: "PUB"}
	rec := postJSON(h.APIGet, `{"uri":"https://example.com/users/u1"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Person")
}

func TestAPIGet_NotFound(t *testing.T) {
	h, _, _, _ := newHandler(t)
	rec := postJSON(h.APIGet, `{"uri":"https://example.com/notes/ghost"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "{}\n", rec.Body.String())
}

func TestAPIGet_InvalidParam(t *testing.T) {
	h, _, _, _ := newHandler(t)
	rec := postJSON(h.APIGet, `{}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAPIGet_UserNotFound(t *testing.T) {
	h, _, _, _ := newHandler(t)
	rec := postJSON(h.APIGet, `{"uri":"https://example.com/users/ghost"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "{}\n", rec.Body.String())
}

func TestAPIGet_UserKeypairError(t *testing.T) {
	h, userRepo, _, keypairRepo := newHandler(t)
	userRepo.Users["u1"] = &model.User{ID: "u1", Username: "alice"}
	keypairRepo.err = errors.New("db down")
	rec := postJSON(h.APIGet, `{"uri":"https://example.com/users/u1"}`)
	// keypairエラー → 空オブジェクト返却
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "{}\n", rec.Body.String())
}

func TestAPIGet_NoMatch(t *testing.T) {
	h, _, _, _ := newHandler(t)
	rec := postJSON(h.APIGet, `{"uri":"https://other.example/something"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAPIShow_Note(t *testing.T) {
	h, _, noteRepo, _ := newHandler(t)
	noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "u1", Visibility: model.NoteVisibilityPublic, User: &model.User{ID: "u1", Username: "alice"}}
	rec := postJSON(h.APIShow, `{"uri":"https://example.com/notes/n1"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"type":"Note"`)
}

func TestAPIShow_User(t *testing.T) {
	h, userRepo, _, _ := newHandler(t)
	userRepo.Users["u1"] = &model.User{ID: "u1", Username: "bob"}
	rec := postJSON(h.APIShow, `{"uri":"https://example.com/users/u1"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"type":"User"`)
}

func TestAPIShow_NotFound(t *testing.T) {
	h, _, _, _ := newHandler(t)
	rec := postJSON(h.APIShow, `{"uri":"https://example.com/notes/ghost"}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAPIShow_InvalidParam(t *testing.T) {
	h, _, _, _ := newHandler(t)
	rec := postJSON(h.APIShow, `{}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAPIShow_RemoteNote(t *testing.T) {
	h, _, noteRepo, _ := newHandler(t)
	host := "remote.example"
	noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "u1", Visibility: model.NoteVisibilityPublic, UserHost: &host}
	rec := postJSON(h.APIShow, `{"uri":"https://example.com/notes/n1"}`)
	// リモートノートはスキップされるので NotFound になる
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAPIShow_RemoteUser(t *testing.T) {
	h, userRepo, _, _ := newHandler(t)
	host := "remote.example"
	userRepo.Users["u1"] = &model.User{ID: "u1", Username: "bob", Host: &host}
	rec := postJSON(h.APIShow, `{"uri":"https://example.com/users/u1"}`)
	// リモートユーザーはスキップされるので NotFound
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestExtractLocalID(t *testing.T) {
	assert.Equal(t, "abc123", extractLocalID("https://example.com/notes/abc123", "/notes/"))
	assert.Equal(t, "xyz", extractLocalID("https://example.com/users/xyz", "/users/"))
	assert.Equal(t, "", extractLocalID("https://example.com/other", "/notes/"))
	assert.Equal(t, "", extractLocalID("", "/notes/"))
}

func TestPackNoteForAPI_NilUser(t *testing.T) {
	n := &model.Note{ID: "n1", UserID: "u1", Visibility: model.NoteVisibilityPublic}
	result := packNoteForAPI(n)
	assert.Equal(t, "n1", result["id"])
	_, hasUser := result["user"]
	assert.False(t, hasUser)
}

func TestPackUserForAPI_Nil(t *testing.T) {
	result := packUserForAPI(nil)
	assert.Empty(t, result)
}

func TestMustMarshal_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	_ = mustMarshal(make(chan int))
}

// --- Remote fetch tests ---

type mockFetcher struct {
	data []byte
	err  error
}

func (m *mockFetcher) FetchObject(_ string) ([]byte, error) {
	return m.data, m.err
}

type mockResolver struct {
	user *model.User
	err  error
}

func (m *mockResolver) ResolveActor(_ string) (*model.User, error) {
	return m.user, m.err
}

func TestSetRemote(t *testing.T) {
	h, _, _, _ := newHandler(t)
	h.SetRemote(&mockFetcher{}, &mockResolver{})
	assert.NotNil(t, h.remoteFetcher)
}

func TestAPNotes(t *testing.T) {
	h, _, _, _ := newHandler(t)
	rec := postJSON(h.APNotes, `{}`)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAPIGet_RemoteFetch(t *testing.T) {
	h, _, _, _ := newHandler(t)
	h.SetRemote(&mockFetcher{data: []byte(`{"type":"Note","id":"https://remote.example/notes/1"}`)}, nil)
	rec := postJSON(h.APIGet, `{"uri":"https://remote.example/notes/1"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "remote.example")
}

func TestAPIGet_RemoteFetchError(t *testing.T) {
	h, _, _, _ := newHandler(t)
	h.SetRemote(&mockFetcher{err: assert.AnError}, nil)
	rec := postJSON(h.APIGet, `{"uri":"https://remote.example/notes/1"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "{}\n", rec.Body.String())
}

func TestAPIShow_RemoteResolveActor(t *testing.T) {
	h, _, _, _ := newHandler(t)
	h.SetRemote(nil, &mockResolver{user: &model.User{ID: "ru1", Username: "remote_alice"}})
	rec := postJSON(h.APIShow, `{"uri":"https://remote.example/users/alice"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "remote_alice")
}

func TestAPIShow_RemoteFetchNote(t *testing.T) {
	h, _, _, _ := newHandler(t)
	h.SetRemote(
		&mockFetcher{data: []byte(`{"type":"Note","content":"hello"}`)},
		&mockResolver{err: assert.AnError},
	)
	rec := postJSON(h.APIShow, `{"uri":"https://remote.example/notes/1"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"type":"Note"`)
}

func TestAPIShow_RemoteNothing(t *testing.T) {
	h, _, _, _ := newHandler(t)
	h.SetRemote(
		&mockFetcher{err: assert.AnError},
		&mockResolver{err: assert.AnError},
	)
	rec := postJSON(h.APIShow, `{"uri":"https://remote.example/unknown"}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAPIGet_RemoteInvalidJSON(t *testing.T) {
	h, _, _, _ := newHandler(t)
	h.SetRemote(&mockFetcher{data: []byte(`not json`)}, nil)
	rec := postJSON(h.APIGet, `{"uri":"https://remote.example/x"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "{}\n", rec.Body.String())
}
