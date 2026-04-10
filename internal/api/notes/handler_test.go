package notes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	corenote "github.com/shiroha-a/mk/internal/core/note"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/server/middleware"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func newTestHandler(t *testing.T) (*Handler, *testutil.MockNoteRepository) {
	t.Helper()
	noteRepo := testutil.NewMockNoteRepository()
	pollRepo := testutil.NewMockPollRepository()
	idGen, _ := id.NewGenerator("aidx")
	createSvc := corenote.NewCreateService(noteRepo, pollRepo, idGen, nil)
	deleteSvc := corenote.NewDeleteService(noteRepo)
	querySvc := corenote.NewQueryService(noteRepo, nil)
	h := NewHandler(noteRepo, createSvc, deleteSvc, querySvc, nil, nil, nil, nil, idGen)
	return h, noteRepo
}

func setAuthUser(c echo.Context, user *model.User) {
	c.Set(string(middleware.UserContextKey), user)
}

func TestCreate_Success(t *testing.T) {
	h, noteRepo := newTestHandler(t)

	user := &model.User{
		ID:                "user1",
		Username:          "testuser",
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}

	body := `{"text": "Hello, world!", "visibility": "public"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/notes/create", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setAuthUser(c, user)

	err := h.Create(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	createdNote, ok := resp["createdNote"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Hello, world!", createdNote["text"])
	assert.Equal(t, "public", createdNote["visibility"])

	// リポジトリにノートが保存されていることを確認
	assert.Len(t, noteRepo.Notes, 1)
}

func TestCreate_EmptyBody(t *testing.T) {
	h, _ := newTestHandler(t)

	user := &model.User{ID: "user1", Username: "testuser"}

	// text, fileIds, renoteIdがすべてない場合
	body := `{}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/notes/create", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setAuthUser(c, user)

	err := h.Create(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCreate_WithPoll(t *testing.T) {
	noteRepo := testutil.NewMockNoteRepository()
	pollRepo := testutil.NewMockPollRepository()
	idGen, _ := id.NewGenerator("aidx")
	createSvc := corenote.NewCreateService(noteRepo, pollRepo, idGen, nil)
	deleteSvc := corenote.NewDeleteService(noteRepo)
	querySvc := corenote.NewQueryService(noteRepo, nil)
	h := NewHandler(noteRepo, createSvc, deleteSvc, querySvc, nil, nil, nil, nil, idGen)

	user := &model.User{
		ID:                "user1",
		Username:          "testuser",
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}

	body := `{"text": "Vote!", "poll": {"choices": ["A", "B", "C"], "multiple": false}}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/notes/create", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setAuthUser(c, user)

	err := h.Create(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Pollがリポジトリに保存されていることを確認
	assert.Len(t, pollRepo.Polls, 1)
}

func TestCreate_DefaultVisibility(t *testing.T) {
	h, noteRepo := newTestHandler(t)

	user := &model.User{
		ID:                "user1",
		Username:          "testuser",
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}

	body := `{"text": "no visibility specified"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/notes/create", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setAuthUser(c, user)

	err := h.Create(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	// デフォルトはpublic
	for _, note := range noteRepo.Notes {
		assert.Equal(t, model.NoteVisibilityPublic, note.Visibility)
	}
}

func TestShow_Success(t *testing.T) {
	h, noteRepo := newTestHandler(t)

	text := "existing note"
	noteRepo.Notes["note1"] = &model.Note{
		ID:         "note1",
		UserID:     "user1",
		Text:       &text,
		Visibility: model.NoteVisibilityPublic,
		Reactions:  datatypes.JSON([]byte("{}")),
		User: &model.User{
			ID:                "user1",
			Username:          "testuser",
			AvatarDecorations: datatypes.JSON([]byte("[]")),
		},
	}

	body := `{"noteId": "note1"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/notes/show", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Show(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "note1", resp["id"])
	assert.Equal(t, "existing note", resp["text"])
}

func TestShow_NotFound(t *testing.T) {
	h, _ := newTestHandler(t)

	body := `{"noteId": "nonexistent"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/notes/show", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Show(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestShow_MissingNoteId(t *testing.T) {
	h, _ := newTestHandler(t)

	body := `{}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/notes/show", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Show(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDelete_Success(t *testing.T) {
	h, noteRepo := newTestHandler(t)

	noteRepo.Notes["note1"] = &model.Note{
		ID:     "note1",
		UserID: "user1",
	}

	user := &model.User{ID: "user1", Username: "testuser"}

	body := `{"noteId": "note1"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/notes/delete", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setAuthUser(c, user)

	err := h.Delete(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, rec.Code)

	// リポジトリから削除されていることを確認
	assert.Len(t, noteRepo.Notes, 0)
}

func TestDelete_NotOwner(t *testing.T) {
	h, noteRepo := newTestHandler(t)

	noteRepo.Notes["note1"] = &model.Note{
		ID:     "note1",
		UserID: "other-user",
	}

	user := &model.User{ID: "user1", Username: "testuser"}

	body := `{"noteId": "note1"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/notes/delete", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setAuthUser(c, user)

	err := h.Delete(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, rec.Code)

	// ノートは削除されていない
	assert.Len(t, noteRepo.Notes, 1)
}

func TestDelete_NotFound(t *testing.T) {
	h, _ := newTestHandler(t)

	user := &model.User{ID: "user1", Username: "testuser"}

	body := `{"noteId": "nonexistent"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/notes/delete", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setAuthUser(c, user)

	err := h.Delete(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestCreate_InvalidJSON(t *testing.T) {
	h, _ := newTestHandler(t)

	user := &model.User{ID: "user1", Username: "testuser"}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/notes/create", strings.NewReader("{invalid"))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setAuthUser(c, user)

	err := h.Create(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCreate_WithVisibleUserIDs(t *testing.T) {
	h, noteRepo := newTestHandler(t)

	user := &model.User{
		ID:                "user1",
		Username:          "testuser",
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}

	body := `{"text": "secret", "visibility": "specified", "visibleUserIds": ["user2", "user3"]}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/notes/create", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setAuthUser(c, user)

	err := h.Create(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	for _, note := range noteRepo.Notes {
		assert.Equal(t, model.NoteVisibility("specified"), note.Visibility)
		assert.Equal(t, []string{"user2", "user3"}, []string(note.VisibleUserIDs))
	}
}

func TestCreate_WithPollExpiresAt(t *testing.T) {
	noteRepo := testutil.NewMockNoteRepository()
	pollRepo := testutil.NewMockPollRepository()
	idGen, _ := id.NewGenerator("aidx")
	createSvc := corenote.NewCreateService(noteRepo, pollRepo, idGen, nil)
	deleteSvc := corenote.NewDeleteService(noteRepo)
	querySvc := corenote.NewQueryService(noteRepo, nil)
	h := NewHandler(noteRepo, createSvc, deleteSvc, querySvc, nil, nil, nil, nil, idGen)

	user := &model.User{
		ID:                "user1",
		Username:          "testuser",
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}

	body := `{"text": "Vote!", "poll": {"choices": ["A", "B"], "multiple": true, "expiresAt": 1700000000000}}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/notes/create", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setAuthUser(c, user)

	err := h.Create(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	for _, poll := range pollRepo.Polls {
		assert.NotNil(t, poll.ExpiresAt)
		assert.True(t, poll.Multiple)
	}
}

func TestCreate_RepoError(t *testing.T) {
	noteRepo := &failingNoteRepo{}
	pollRepo := testutil.NewMockPollRepository()
	idGen, _ := id.NewGenerator("aidx")
	createSvc := corenote.NewCreateService(noteRepo, pollRepo, idGen, nil)
	deleteSvc := corenote.NewDeleteService(noteRepo)
	querySvc := corenote.NewQueryService(noteRepo, nil)
	h := NewHandler(noteRepo, createSvc, deleteSvc, querySvc, nil, nil, nil, nil, idGen)

	user := &model.User{ID: "user1", Username: "testuser"}

	body := `{"text": "will fail"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/notes/create", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setAuthUser(c, user)

	err := h.Create(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestCreate_FindByIDWithUserFails(t *testing.T) {
	noteRepo := &findFailNoteRepo{MockNoteRepository: testutil.NewMockNoteRepository()}
	pollRepo := testutil.NewMockPollRepository()
	idGen, _ := id.NewGenerator("aidx")
	createSvc := corenote.NewCreateService(noteRepo, pollRepo, idGen, nil)
	deleteSvc := corenote.NewDeleteService(noteRepo)
	querySvc := corenote.NewQueryService(noteRepo, nil)
	h := NewHandler(noteRepo, createSvc, deleteSvc, querySvc, nil, nil, nil, nil, idGen)

	user := &model.User{
		ID:                "user1",
		Username:          "testuser",
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}

	body := `{"text": "fallback path"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/notes/create", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setAuthUser(c, user)

	err := h.Create(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestDelete_InvalidJSON(t *testing.T) {
	h, _ := newTestHandler(t)

	user := &model.User{ID: "user1", Username: "testuser"}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/notes/delete", strings.NewReader("{invalid"))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setAuthUser(c, user)

	err := h.Delete(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// failingNoteRepo always returns errors on Create.
type failingNoteRepo struct{}

func (f *failingNoteRepo) Create(_ *model.Note) error             { return testutil.ErrNotFound }
func (f *failingNoteRepo) FindByID(_ string) (*model.Note, error) { return nil, testutil.ErrNotFound }
func (f *failingNoteRepo) FindByIDWithUser(_ string) (*model.Note, error) {
	return nil, testutil.ErrNotFound
}
func (f *failingNoteRepo) FindByURI(_ string) (*model.Note, error) {
	return nil, testutil.ErrNotFound
}
func (f *failingNoteRepo) Delete(_ *model.Note) error                        { return nil }
func (f *failingNoteRepo) Update(_ *model.Note, _ string, _ any) error       { return nil }
func (f *failingNoteRepo) UpdateFields(_ string, _ map[string]any) error     { return nil }
func (f *failingNoteRepo) IncrementCount(_ string, _ string, _ int) error    { return nil }
func (f *failingNoteRepo) IncrementReaction(_ string, _ string, _ int) error { return nil }
func (f *failingNoteRepo) ListByUserID(_ string, _, _ string, _ int) ([]*model.Note, error) {
	return nil, nil
}
func (f *failingNoteRepo) ListByChannelID(_ string, _, _ string, _ int) ([]*model.Note, error) {
	return nil, nil
}
func (f *failingNoteRepo) FindManyByIDsWithUser(_ []string) ([]*model.Note, error) {
	return nil, nil
}
func (f *failingNoteRepo) ListRenotesOf(_ string, _, _ string, _ int) ([]*model.Note, error) {
	return nil, nil
}
func (f *failingNoteRepo) ListRepliesOf(_ string, _, _ string, _ int) ([]*model.Note, error) {
	return nil, nil
}
func (f *failingNoteRepo) ListChildrenOf(_ string, _, _ string, _ int) ([]*model.Note, error) {
	return nil, nil
}
func (f *failingNoteRepo) SearchByFilter(_ model.NoteSearchFilter) ([]*model.Note, error) {
	return nil, nil
}
func (f *failingNoteRepo) ListFeatured(_, _ int) ([]*model.Note, error) {
	return nil, nil
}
func (f *failingNoteRepo) FindRenoteByUser(_, _ string) (*model.Note, error) {
	return nil, testutil.ErrNotFound
}
func (f *failingNoteRepo) ListMentions(_ string, _ int, _, _ string) ([]*model.Note, error) {
	return nil, nil
}
func (f *failingNoteRepo) SearchByTag(_ string, _ int, _, _ string) ([]*model.Note, error) {
	return nil, nil
}

// findFailNoteRepo creates successfully but FindByIDWithUser always fails.
type findFailNoteRepo struct {
	*testutil.MockNoteRepository
}

func (f *findFailNoteRepo) FindByIDWithUser(_ string) (*model.Note, error) {
	return nil, testutil.ErrNotFound
}

// deleteFailNoteRepo finds a note successfully but fails on Delete, used to
// trigger the handler's default error path.
type deleteFailNoteRepo struct {
	*testutil.MockNoteRepository
}

func (f *deleteFailNoteRepo) Delete(_ *model.Note) error { return testutil.ErrNotFound }

func TestDelete_RepoError(t *testing.T) {
	mockRepo := testutil.NewMockNoteRepository()
	mockRepo.Notes["note1"] = &model.Note{ID: "note1", UserID: "user1"}
	noteRepo := &deleteFailNoteRepo{MockNoteRepository: mockRepo}
	pollRepo := testutil.NewMockPollRepository()
	idGen, _ := id.NewGenerator("aidx")
	createSvc := corenote.NewCreateService(noteRepo, pollRepo, idGen, nil)
	deleteSvc := corenote.NewDeleteService(noteRepo)
	querySvc := corenote.NewQueryService(noteRepo, nil)
	h := NewHandler(noteRepo, createSvc, deleteSvc, querySvc, nil, nil, nil, nil, idGen)

	user := &model.User{ID: "user1", Username: "testuser"}
	body := `{"noteId": "note1"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/notes/delete", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setAuthUser(c, user)

	err := h.Delete(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestExtractMentions(t *testing.T) {
	tests := []struct {
		text     string
		expected []string
	}{
		{"Hello @alice @bob", []string{"alice", "bob"}},
		{"No mentions here", nil},
		{"@single", []string{"single"}},
		{"@", nil},
		{"", nil},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			result := corenote.ExtractMentions(tt.text)
			assert.Equal(t, tt.expected, result)
		})
	}
}
