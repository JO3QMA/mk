package users

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	corefollowing "github.com/shiroha-a/mk/internal/core/following"
	coreuser "github.com/shiroha-a/mk/internal/core/user"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func newTestHandler(t *testing.T) (*Handler, *testutil.MockUserRepository) {
	t.Helper()
	userRepo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	piningRepo := testutil.NewMockUserNotePiningRepository()
	fRepo := testutil.NewMockFollowingRepository()
	frRepo := testutil.NewMockFollowRequestRepository()
	idGen, _ := id.NewGenerator("aidx")
	svc := coreuser.NewService(userRepo, noteRepo, piningRepo, idGen)
	fSvc := corefollowing.NewService(userRepo, fRepo, frRepo, idGen)
	h := NewHandler(svc, fSvc, noteRepo, idGen)
	return h, userRepo
}

func addTestUser(repo *testutil.MockUserRepository) *model.User {
	name := "Test User"
	user := &model.User{
		ID:                "user1",
		Username:          "testuser",
		UsernameLower:     "testuser",
		Name:              &name,
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}
	repo.Users["user1"] = user
	return user
}

func TestShow_ByUserID(t *testing.T) {
	h, userRepo := newTestHandler(t)
	addTestUser(userRepo)

	body := `{"userId": "user1"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/users/show", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Show(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "user1", resp["id"])
	assert.Equal(t, "testuser", resp["username"])
	assert.Equal(t, "Test User", resp["name"])
}

func TestShow_ByUsername(t *testing.T) {
	h, userRepo := newTestHandler(t)
	addTestUser(userRepo)

	body := `{"username": "testuser"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/users/show", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Show(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "user1", resp["id"])
}

func TestShow_ByUsernameWithHost(t *testing.T) {
	h, userRepo := newTestHandler(t)

	host := "remote.example.com"
	user := &model.User{
		ID:                "user2",
		Username:          "remoteuser",
		UsernameLower:     "remoteuser",
		Host:              &host,
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}
	userRepo.Users["user2"] = user

	body := `{"username": "remoteuser", "host": "remote.example.com"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/users/show", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Show(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "user2", resp["id"])
}

func TestShow_NotFound(t *testing.T) {
	h, _ := newTestHandler(t)

	body := `{"userId": "nonexistent"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/users/show", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Show(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestShow_MissingParams(t *testing.T) {
	h, _ := newTestHandler(t)

	body := `{}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/users/show", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Show(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestShow_WithProfile(t *testing.T) {
	h, userRepo := newTestHandler(t)
	addTestUser(userRepo)

	desc := "Hello, I'm a test user"
	location := "Tokyo"
	userRepo.Profiles["user1"] = &model.UserProfile{
		UserID:      "user1",
		Description: &desc,
		Location:    &location,
		Fields:      datatypes.JSON([]byte("[]")),
	}

	body := `{"userId": "user1"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/users/show", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Show(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "Hello, I'm a test user", resp["description"])
	assert.Equal(t, "Tokyo", resp["location"])
}

func TestShow_InvalidJSON(t *testing.T) {
	h, _ := newTestHandler(t)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/users/show", strings.NewReader("{invalid"))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Show(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestShow_UsernameNotFound(t *testing.T) {
	h, _ := newTestHandler(t)

	body := `{"username": "nonexistent"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/users/show", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Show(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// --- post is a small helper that exercises a handler with an optional body ---

func post(h echo.HandlerFunc, body string) *httptest.ResponseRecorder {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	_ = h(c)
	return rec
}

// --- Search ---

func TestSearch_Success(t *testing.T) {
	h, repo := newTestHandler(t)
	addTestUser(repo)

	rec := post(h.Search, `{"query": "test"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	var out []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Len(t, out, 1)
}

func TestSearch_DefaultLimit(t *testing.T) {
	h, repo := newTestHandler(t)
	addTestUser(repo)

	rec := post(h.Search, `{"query": "test", "limit": 0}`)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestSearch_InvalidJSON(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := post(h.Search, `{invalid`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Notes ---

func TestNotes_Success(t *testing.T) {
	h, repo := newTestHandler(t)
	addTestUser(repo)
	// Insert a note via the noteRepo embedded in handler
	idGen, _ := id.NewGenerator("aidx")
	noteRepo := h.noteRepo.(*testutil.MockNoteRepository)
	noteID := idGen.Generate(time.Now())
	text := "hello"
	noteRepo.Notes[noteID] = &model.Note{
		ID:         noteID,
		UserID:     "user1",
		Text:       &text,
		Visibility: model.NoteVisibilityPublic,
		Reactions:  datatypes.JSON([]byte("{}")),
	}

	rec := post(h.Notes, `{"userId": "user1"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	var out []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Len(t, out, 1)
}

func TestNotes_LimitClamp(t *testing.T) {
	h, repo := newTestHandler(t)
	addTestUser(repo)
	rec := post(h.Notes, `{"userId": "user1", "limit": 9999}`)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestNotes_UserNotFound(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := post(h.Notes, `{"userId": "ghost"}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestNotes_InvalidParam(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := post(h.Notes, `{}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Followers / Following ---

func TestFollowers_Success(t *testing.T) {
	h, repo := newTestHandler(t)
	addTestUser(repo)
	// Add a follower relationship
	repo.Users["follower1"] = &model.User{
		ID:                "follower1",
		Username:          "follower1",
		UsernameLower:     "follower1",
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}
	fSvc := h.followingService
	_, err := fSvc.Follow("follower1", "user1")
	require.NoError(t, err)

	rec := post(h.Followers, `{"userId": "user1"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	var out []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Len(t, out, 1)
}

func TestFollowers_LimitClamp(t *testing.T) {
	h, repo := newTestHandler(t)
	addTestUser(repo)
	rec := post(h.Followers, `{"userId": "user1", "limit": 9999}`)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestFollowers_UserNotFound(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := post(h.Followers, `{"userId": "ghost"}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestFollowers_InvalidParam(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := post(h.Followers, `{}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestFollowing_Success(t *testing.T) {
	h, repo := newTestHandler(t)
	addTestUser(repo)
	repo.Users["followee1"] = &model.User{
		ID:                "followee1",
		Username:          "followee1",
		UsernameLower:     "followee1",
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}
	_, err := h.followingService.Follow("user1", "followee1")
	require.NoError(t, err)

	rec := post(h.Following, `{"userId": "user1"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	var out []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Len(t, out, 1)
}

func TestFollowing_InvalidParam(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := post(h.Following, `{}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Internal error paths via failing repos ---

type failingNoteRepo struct {
	*testutil.MockNoteRepository
}

func (f *failingNoteRepo) ListByUserID(_ string, _, _ string, _ int) ([]*model.Note, error) {
	return nil, assertErr
}

type failingUserRepo struct {
	*testutil.MockUserRepository
}

func (f *failingUserRepo) SearchByUsername(_ string, _, _ int) ([]*model.User, error) {
	return nil, assertErr
}

type failingFollowingRepo struct {
	*testutil.MockFollowingRepository
}

func (f *failingFollowingRepo) ListFollowers(_ string, _, _ int) ([]*model.Following, error) {
	return nil, assertErr
}

func (f *failingFollowingRepo) ListFollowing(_ string, _, _ int) ([]*model.Following, error) {
	return nil, assertErr
}

var assertErr = &simpleErr{"stub"}

type simpleErr struct{ msg string }

func (e *simpleErr) Error() string { return e.msg }

func newHandlerWithFailingNoteRepo(t *testing.T) *Handler {
	t.Helper()
	userRepo := testutil.NewMockUserRepository()
	noteRepo := &failingNoteRepo{MockNoteRepository: testutil.NewMockNoteRepository()}
	piningRepo := testutil.NewMockUserNotePiningRepository()
	fRepo := testutil.NewMockFollowingRepository()
	frRepo := testutil.NewMockFollowRequestRepository()
	idGen, _ := id.NewGenerator("aidx")
	svc := coreuser.NewService(userRepo, noteRepo, piningRepo, idGen)
	fSvc := corefollowing.NewService(userRepo, fRepo, frRepo, idGen)
	addTestUser(userRepo)
	return NewHandler(svc, fSvc, noteRepo, idGen)
}

func newHandlerWithFailingSearch(t *testing.T) *Handler {
	t.Helper()
	mockUR := testutil.NewMockUserRepository()
	addTestUser(mockUR)
	userRepo := &failingUserRepo{MockUserRepository: mockUR}
	noteRepo := testutil.NewMockNoteRepository()
	piningRepo := testutil.NewMockUserNotePiningRepository()
	fRepo := testutil.NewMockFollowingRepository()
	frRepo := testutil.NewMockFollowRequestRepository()
	idGen, _ := id.NewGenerator("aidx")
	svc := coreuser.NewService(userRepo, noteRepo, piningRepo, idGen)
	fSvc := corefollowing.NewService(userRepo, fRepo, frRepo, idGen)
	return NewHandler(svc, fSvc, noteRepo, idGen)
}

func newHandlerWithFailingFollowing(t *testing.T) *Handler {
	t.Helper()
	userRepo := testutil.NewMockUserRepository()
	addTestUser(userRepo)
	noteRepo := testutil.NewMockNoteRepository()
	piningRepo := testutil.NewMockUserNotePiningRepository()
	fRepo := &failingFollowingRepo{MockFollowingRepository: testutil.NewMockFollowingRepository()}
	frRepo := testutil.NewMockFollowRequestRepository()
	idGen, _ := id.NewGenerator("aidx")
	svc := coreuser.NewService(userRepo, noteRepo, piningRepo, idGen)
	fSvc := corefollowing.NewService(userRepo, fRepo, frRepo, idGen)
	return NewHandler(svc, fSvc, noteRepo, idGen)
}

func TestSearch_InternalError(t *testing.T) {
	h := newHandlerWithFailingSearch(t)
	rec := post(h.Search, `{"query": "test"}`)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestNotes_InternalError(t *testing.T) {
	h := newHandlerWithFailingNoteRepo(t)
	rec := post(h.Notes, `{"userId": "user1"}`)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestFollowers_InternalError(t *testing.T) {
	h := newHandlerWithFailingFollowing(t)
	rec := post(h.Followers, `{"userId": "user1"}`)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestFollowing_InternalError(t *testing.T) {
	h := newHandlerWithFailingFollowing(t)
	rec := post(h.Following, `{"userId": "user1"}`)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
