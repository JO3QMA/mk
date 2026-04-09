package i

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

	// Phase 4.5c 互換性フィールド
	assert.Equal(t, false, resp["isAdmin"])
	assert.Equal(t, false, resp["isModerator"])
	assert.Equal(t, false, resp["isDeleted"])
	assert.NotNil(t, resp["pinnedNoteIds"])
	assert.NotNil(t, resp["pinnedNotes"])
	assert.Nil(t, resp["pinnedPageId"])
	assert.NotNil(t, resp["policies"])
	assert.NotNil(t, resp["roles"])
	assert.NotNil(t, resp["achievements"])
	assert.NotNil(t, resp["unreadAnnouncements"])
	assert.Equal(t, false, resp["publicReactions"]) // profile has default false
	// C3 追加フィールド
	assert.NotNil(t, resp["avatarUrl"]) // identicon URL 自動生成
	assert.Equal(t, false, resp["hasUnreadChatMessages"])
	assert.Equal(t, "public", resp["followersVisibility"])
	assert.Equal(t, "public", resp["followingVisibility"])
	assert.Equal(t, "mutual", resp["chatScope"])
	assert.Equal(t, true, resp["canChat"])
	assert.NotNil(t, resp["verifiedLinks"])
	assert.NotNil(t, resp["securityKeysList"])
	assert.NotNil(t, resp["mutingNotificationTypes"])
	assert.Equal(t, false, resp["securityKeys"])
	assert.Nil(t, resp["movedTo"])
	assert.Nil(t, resp["alsoKnownAs"])
}

// stubRoleProvider implements i.RoleProvider for testing.
type stubRoleProvider struct {
	admin     bool
	moderator bool
	roles     []*model.Role
	policies  map[string]any
}

func (s *stubRoleProvider) IsAdministrator(_ string) bool { return s.admin }
func (s *stubRoleProvider) IsModerator(_ string) bool     { return s.moderator }
func (s *stubRoleProvider) GetUserRoles(_ string) ([]*model.Role, error) {
	return s.roles, nil
}
func (s *stubRoleProvider) GetUserPolicies(_ string) map[string]any {
	if s.policies != nil {
		return s.policies
	}
	return map[string]any{}
}

func TestMe_WithRoleProvider(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetRoleProvider(&stubRoleProvider{
		admin:     true,
		moderator: true,
		roles: []*model.Role{
			{ID: "r1", Name: "Admin", IsAdministrator: true, DisplayOrder: 10},
		},
		policies: map[string]any{"gtlAvailable": true, "driveCapacityMb": 500},
	})

	user := &model.User{
		ID:                "user1",
		Username:          "admin",
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/i", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(string(middleware.UserContextKey), user)

	err := h.Me(c)
	require.NoError(t, err)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["isAdmin"])
	assert.Equal(t, true, resp["isModerator"])

	roles := resp["roles"].([]any)
	assert.Len(t, roles, 1)
	role := roles[0].(map[string]any)
	assert.Equal(t, "Admin", role["name"])

	policies := resp["policies"].(map[string]any)
	assert.Equal(t, float64(500), policies["driveCapacityMb"])
}

func TestMe_CreatedAtFromValidID(t *testing.T) {
	h, _, _, _ := newTestHandler(t)

	// AIDXで生成した有効なIDを使う
	idGen, _ := id.NewGenerator("aidx")
	validID := idGen.Generate(java_time())

	user := &model.User{
		ID:                validID,
		Username:          "validid",
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/i", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(string(middleware.UserContextKey), user)

	err := h.Me(c)
	require.NoError(t, err)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	// createdAt は有効なIDから復元される
	createdAt, ok := resp["createdAt"].(string)
	require.True(t, ok, "createdAt should be a string")
	assert.Contains(t, createdAt, "T") // ISO8601 format
}

func java_time() time.Time {
	return time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
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

// --- Registry endpoints ---

func newHandlerWithRegistry(t *testing.T) (*Handler, *testutil.MockRegistryRepository) {
	t.Helper()
	h, _, _, _ := newTestHandler(t)
	regRepo := testutil.NewMockRegistryRepository()
	h.SetRegistryRepo(regRepo)
	return h, regRepo
}

func TestRegistrySet_Success(t *testing.T) {
	h, regRepo := newHandlerWithRegistry(t)
	user := &model.User{ID: "u1"}
	rec := post(h.RegistrySet, `{"key":"theme","value":"dark"}`, user)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Len(t, regRepo.Items, 1)
}

func TestRegistrySet_InvalidParam(t *testing.T) {
	h, _ := newHandlerWithRegistry(t)
	rec := post(h.RegistrySet, `{}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRegistryGet_Success(t *testing.T) {
	h, _ := newHandlerWithRegistry(t)
	user := &model.User{ID: "u1"}
	post(h.RegistrySet, `{"key":"lang","value":"ja"}`, user)
	rec := post(h.RegistryGet, `{"key":"lang"}`, user)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRegistryGet_NotFound(t *testing.T) {
	h, _ := newHandlerWithRegistry(t)
	rec := post(h.RegistryGet, `{"key":"ghost"}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRegistryGet_InvalidParam(t *testing.T) {
	h, _ := newHandlerWithRegistry(t)
	rec := post(h.RegistryGet, `{}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRegistryGetAll_Success(t *testing.T) {
	h, _ := newHandlerWithRegistry(t)
	user := &model.User{ID: "u1"}
	post(h.RegistrySet, `{"key":"k1","value":1}`, user)
	post(h.RegistrySet, `{"key":"k2","value":2}`, user)
	rec := post(h.RegistryGetAll, `{}`, user)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRegistryGetAll_InvalidJSON(t *testing.T) {
	h, _ := newHandlerWithRegistry(t)
	rec := post(h.RegistryGetAll, `invalid`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRegistryKeysWithType_Success(t *testing.T) {
	h, _ := newHandlerWithRegistry(t)
	user := &model.User{ID: "u1"}
	post(h.RegistrySet, `{"key":"theme","value":"dark"}`, user)
	rec := post(h.RegistryKeysWithType, `{}`, user)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRegistryKeysWithType_InvalidJSON(t *testing.T) {
	h, _ := newHandlerWithRegistry(t)
	rec := post(h.RegistryKeysWithType, `invalid`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRegistryRemove_Success(t *testing.T) {
	h, regRepo := newHandlerWithRegistry(t)
	user := &model.User{ID: "u1"}
	post(h.RegistrySet, `{"key":"temp","value":true}`, user)
	assert.Len(t, regRepo.Items, 1)
	rec := post(h.RegistryRemove, `{"key":"temp"}`, user)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, regRepo.Items)
}

type failingSetRegistryRepo struct {
	*testutil.MockRegistryRepository
}

func (f *failingSetRegistryRepo) Set(_ *model.RegistryItem) error { return stubError }

type failingGetAllRegistryRepo struct {
	*testutil.MockRegistryRepository
}

func (f *failingGetAllRegistryRepo) GetAll(_ string, _ []string, _ *string) ([]*model.RegistryItem, error) {
	return nil, stubError
}

func (f *failingGetAllRegistryRepo) KeysWithType(_ string, _ []string, _ *string) (map[string]string, error) {
	return nil, stubError
}

type failingRemoveRegistryRepo struct {
	*testutil.MockRegistryRepository
}

func (f *failingRemoveRegistryRepo) Remove(_ string, _ string, _ []string, _ *string) error {
	return stubError
}

func TestRegistrySet_Error(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetRegistryRepo(&failingSetRegistryRepo{testutil.NewMockRegistryRepository()})
	rec := post(h.RegistrySet, `{"key":"k","value":1}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestRegistryGetAll_Error(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetRegistryRepo(&failingGetAllRegistryRepo{testutil.NewMockRegistryRepository()})
	rec := post(h.RegistryGetAll, `{}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestRegistryKeysWithType_Error(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetRegistryRepo(&failingGetAllRegistryRepo{testutil.NewMockRegistryRepository()})
	rec := post(h.RegistryKeysWithType, `{}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestRegistryRemove_Error(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetRegistryRepo(&failingRemoveRegistryRepo{testutil.NewMockRegistryRepository()})
	rec := post(h.RegistryRemove, `{"key":"k"}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestRegistryRemove_InvalidParam(t *testing.T) {
	h, _ := newHandlerWithRegistry(t)
	rec := post(h.RegistryRemove, `{}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
