package i

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	coreuser "github.com/shiroha-a/mk/internal/core/user"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/server/middleware"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

var stubError = errors.New("stub error")

// failingPiningRepo lets us trigger non-domain errors from PinNote.
type failingPiningRepo struct {
	*testutil.MockUserNotePiningRepository
}

func (f *failingPiningRepo) CountByUser(_ string) (int, error) { return 0, stubError }

// failingPiningDeleteRepo lets us trigger non-domain errors from UnpinNote
// while still allowing FindByPair to succeed (so the service reaches Delete).
type failingPiningDeleteRepo struct {
	*testutil.MockUserNotePiningRepository
}

func (f *failingPiningDeleteRepo) Delete(_ *model.UserNotePining) error {
	return stubError
}

func newHandlerWithFailingUnpinDelete(t *testing.T) (*Handler, *testutil.MockUserNotePiningRepository) {
	t.Helper()
	userRepo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	mock := testutil.NewMockUserNotePiningRepository()
	piningRepo := &failingPiningDeleteRepo{MockUserNotePiningRepository: mock}
	idGen, _ := id.NewGenerator("aidx")
	svc := coreuser.NewService(userRepo, noteRepo, piningRepo, idGen)
	return NewHandler(svc, idGen), mock
}

func newHandlerWithFailingPiningCount(t *testing.T) (*Handler, *testutil.MockUserRepository, *testutil.MockNoteRepository) {
	t.Helper()
	userRepo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	piningRepo := &failingPiningRepo{MockUserNotePiningRepository: testutil.NewMockUserNotePiningRepository()}
	idGen, _ := id.NewGenerator("aidx")
	svc := coreuser.NewService(userRepo, noteRepo, piningRepo, idGen)
	return NewHandler(svc, idGen), userRepo, noteRepo
}

// failingUserRepoForUpdate forces user.UpdateUser to fail.
type failingUserRepoForUpdate struct {
	*testutil.MockUserRepository
}

func (f *failingUserRepoForUpdate) UpdateUser(_ string, _ map[string]any) error { return stubError }

func newHandlerWithFailingUpdate(t *testing.T) (*Handler, *testutil.MockUserRepository) {
	t.Helper()
	mockUR := testutil.NewMockUserRepository()
	mockUR.Users["user1"] = &model.User{
		ID:                "user1",
		Username:          "user1",
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}
	userRepo := &failingUserRepoForUpdate{MockUserRepository: mockUR}
	noteRepo := testutil.NewMockNoteRepository()
	piningRepo := testutil.NewMockUserNotePiningRepository()
	idGen, _ := id.NewGenerator("aidx")
	svc := coreuser.NewService(userRepo, noteRepo, piningRepo, idGen)
	return NewHandler(svc, idGen), mockUR
}

// post is a small helper to invoke a handler with an authenticated request.
func post(h echo.HandlerFunc, body string, me *model.User) *httptest.ResponseRecorder {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if me != nil {
		c.Set(string(middleware.UserContextKey), me)
	}
	_ = h(c)
	return rec
}

func newTestHandler(t *testing.T) (*Handler, *testutil.MockUserRepository, *testutil.MockNoteRepository, *testutil.MockUserNotePiningRepository) {
	t.Helper()
	userRepo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	piningRepo := testutil.NewMockUserNotePiningRepository()
	idGen, _ := id.NewGenerator("aidx")
	svc := coreuser.NewService(userRepo, noteRepo, piningRepo, idGen)
	h := NewHandler(svc, idGen)
	return h, userRepo, noteRepo, piningRepo
}

func TestMe_Success(t *testing.T) {
	h, userRepo, _, _ := newTestHandler(t)

	name := "Test User"
	user := &model.User{
		ID:                "user1",
		Username:          "testuser",
		Name:              &name,
		FollowersCount:    10,
		FollowingCount:    20,
		NotesCount:        100,
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}

	email := "test@example.com"
	userRepo.Profiles["user1"] = &model.UserProfile{
		UserID:             "user1",
		Email:              &email,
		EmailVerified:      true,
		TwoFactorEnabled:   false,
		AutoAcceptFollowed: true,
		NoCrawle:           false,
		PreventAiLearning:  true,
		Fields:             datatypes.JSON([]byte("[]")),
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/i", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(string(middleware.UserContextKey), user)

	err := h.Me(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	assert.Equal(t, "user1", resp["id"])
	assert.Equal(t, "testuser", resp["username"])
	assert.Equal(t, "Test User", resp["name"])
	assert.Equal(t, float64(10), resp["followersCount"])
	assert.Equal(t, float64(20), resp["followingCount"])
	assert.Equal(t, float64(100), resp["notesCount"])

	// Private fields
	assert.Equal(t, "test@example.com", resp["email"])
	assert.Equal(t, true, resp["emailVerified"])
	assert.Equal(t, true, resp["autoAcceptFollowed"])
	assert.Equal(t, false, resp["twoFactorEnabled"])
	assert.Equal(t, true, resp["preventAiLearning"])

	// Hardcoded fields
	assert.Equal(t, false, resp["hasUnreadNotification"])
	assert.Equal(t, false, resp["hasPendingReceivedFollowRequest"])
}

func TestMe_NoProfile(t *testing.T) {
	h, _, _, _ := newTestHandler(t)

	user := &model.User{
		ID:                "user1",
		Username:          "noprofile",
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/i", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(string(middleware.UserContextKey), user)

	err := h.Me(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	assert.Equal(t, "user1", resp["id"])
	assert.Equal(t, "noprofile", resp["username"])
	// profileがない場合、private fieldsはレスポンスに含まれない
	assert.Nil(t, resp["email"])
	// ただしhardcoded fieldsは含まれる
	assert.Equal(t, false, resp["hasUnreadNotification"])
}

// --- Update ---

func TestUpdate_Success(t *testing.T) {
	h, repo, _, _ := newTestHandler(t)
	user := &model.User{
		ID:                "user1",
		Username:          "user1",
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}
	repo.Users["user1"] = user

	rec := post(h.Update, `{"name": "New Name", "description": "hi", "location": "Tokyo", "birthday": "1990-01-01", "lang": "ja", "isLocked": true, "isBot": true, "isCat": true, "isExplorable": false, "hideOnlineStatus": true, "alwaysMarkNsfw": true, "autoSensitive": true, "noCrawle": true, "preventAiLearning": true}`, user)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestUpdate_InvalidJSON(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	user := &model.User{ID: "user1"}
	rec := post(h.Update, `{invalid`, user)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUpdate_UserNotFound(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	user := &model.User{ID: "ghost"}
	rec := post(h.Update, `{}`, user)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUpdate_InternalError(t *testing.T) {
	h, _ := newHandlerWithFailingUpdate(t)
	user := &model.User{ID: "user1"}
	rec := post(h.Update, `{"isLocked": true}`, user)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- Pin ---

func TestPin_Success(t *testing.T) {
	h, repo, noteRepo, _ := newTestHandler(t)
	user := &model.User{
		ID:                "user1",
		Username:          "user1",
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}
	repo.Users["user1"] = user
	noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "user1"}

	rec := post(h.Pin, `{"noteId": "n1"}`, user)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestPin_NoteNotFound(t *testing.T) {
	h, repo, _, _ := newTestHandler(t)
	user := &model.User{ID: "user1", AvatarDecorations: datatypes.JSON([]byte("[]"))}
	repo.Users["user1"] = user

	rec := post(h.Pin, `{"noteId": "ghost"}`, user)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestPin_AlreadyPinned(t *testing.T) {
	h, repo, noteRepo, _ := newTestHandler(t)
	user := &model.User{ID: "user1", AvatarDecorations: datatypes.JSON([]byte("[]"))}
	repo.Users["user1"] = user
	noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "user1"}
	post(h.Pin, `{"noteId": "n1"}`, user)

	rec := post(h.Pin, `{"noteId": "n1"}`, user)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPin_LimitExceeded(t *testing.T) {
	h, repo, noteRepo, _ := newTestHandler(t)
	user := &model.User{ID: "user1", AvatarDecorations: datatypes.JSON([]byte("[]"))}
	repo.Users["user1"] = user
	for i := 1; i <= coreuser.MaxPinnedNotes; i++ {
		nid := "n" + string(rune('0'+i))
		noteRepo.Notes[nid] = &model.Note{ID: nid, UserID: "user1"}
		post(h.Pin, `{"noteId": "`+nid+`"}`, user)
	}
	noteRepo.Notes["nx"] = &model.Note{ID: "nx", UserID: "user1"}

	rec := post(h.Pin, `{"noteId": "nx"}`, user)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPin_InvalidParam(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	user := &model.User{ID: "user1"}
	rec := post(h.Pin, `{}`, user)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPin_InternalError(t *testing.T) {
	h, repo, noteRepo := newHandlerWithFailingPiningCount(t)
	user := &model.User{ID: "user1", AvatarDecorations: datatypes.JSON([]byte("[]"))}
	repo.Users["user1"] = user
	noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "user1"}

	rec := post(h.Pin, `{"noteId": "n1"}`, user)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- Unpin ---

func TestUnpin_Success(t *testing.T) {
	h, repo, noteRepo, _ := newTestHandler(t)
	user := &model.User{ID: "user1", AvatarDecorations: datatypes.JSON([]byte("[]"))}
	repo.Users["user1"] = user
	noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "user1"}
	post(h.Pin, `{"noteId": "n1"}`, user)

	rec := post(h.Unpin, `{"noteId": "n1"}`, user)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestUnpin_NotFound(t *testing.T) {
	h, repo, _, _ := newTestHandler(t)
	user := &model.User{ID: "user1", AvatarDecorations: datatypes.JSON([]byte("[]"))}
	repo.Users["user1"] = user

	rec := post(h.Unpin, `{"noteId": "n1"}`, user)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUnpin_InvalidParam(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	user := &model.User{ID: "user1"}
	rec := post(h.Unpin, `{}`, user)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUnpin_InternalError(t *testing.T) {
	h, mock := newHandlerWithFailingUnpinDelete(t)
	mock.Pinings["p1"] = &model.UserNotePining{ID: "p1", UserID: "user1", NoteID: "n1"}
	user := &model.User{ID: "user1"}

	rec := post(h.Unpin, `{"noteId": "n1"}`, user)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
