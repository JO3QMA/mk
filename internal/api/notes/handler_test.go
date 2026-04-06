package notes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/misskey-dev/misskey-go/internal/misc/id"
	"github.com/misskey-dev/misskey-go/internal/model"
	"github.com/misskey-dev/misskey-go/internal/server/middleware"
	"github.com/misskey-dev/misskey-go/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func newTestHandler(t *testing.T) (*Handler, *testutil.MockNoteRepository) {
	t.Helper()
	noteRepo := testutil.NewMockNoteRepository()
	pollRepo := testutil.NewMockPollRepository()
	idGen, _ := id.NewGenerator("aidx")
	h := NewHandler(noteRepo, pollRepo, idGen)
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
	h := NewHandler(noteRepo, pollRepo, idGen)

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
			result := extractMentions(tt.text)
			assert.Equal(t, tt.expected, result)
		})
	}
}
