package notes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	corenote "github.com/shiroha-a/mk/internal/core/note"
	"github.com/shiroha-a/mk/internal/core/search"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func newQueryHandler(t *testing.T) (*Handler, *testutil.MockNoteRepository) {
	t.Helper()
	noteRepo := testutil.NewMockNoteRepository()
	pollRepo := testutil.NewMockPollRepository()
	idGen, _ := id.NewGenerator("aidx")
	createSvc := corenote.NewCreateService(noteRepo, pollRepo, idGen, nil)
	deleteSvc := corenote.NewDeleteService(noteRepo)
	querySvc := corenote.NewQueryService(noteRepo, nil)
	searchSvc := search.NewService(search.NewSQLLikeProvider(noteRepo, nil))
	h := NewHandler(noteRepo, createSvc, deleteSvc, querySvc, nil, nil, nil, searchSvc, idGen)
	return h, noteRepo
}

func newJSONRequest(t *testing.T, path, body string) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

// putUserOnNote stores a populated user on the note so PackNote serializes correctly.
func putUserOnNote(n *model.Note) {
	n.User = &model.User{
		ID:                n.UserID,
		Username:          "u",
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}
}

func seedPublicNote(repo *testutil.MockNoteRepository, id string) *model.Note {
	n := &model.Note{
		ID:         id,
		UserID:     "author",
		Visibility: model.NoteVisibilityPublic,
		Reactions:  datatypes.JSON([]byte("{}")),
	}
	putUserOnNote(n)
	repo.Notes[id] = n
	return n
}

func TestRenotes_OK(t *testing.T) {
	h, repo := newQueryHandler(t)
	parent := seedPublicNote(repo, "parent")
	pid := parent.ID
	r := seedPublicNote(repo, "r1")
	r.RenoteID = &pid

	c, rec := newJSONRequest(t, "/api/notes/renotes", `{"noteId":"parent"}`)
	require.NoError(t, h.Renotes(c))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp, 1)
	assert.Equal(t, "r1", resp[0]["id"])
}

func TestRenotes_NotFound(t *testing.T) {
	h, _ := newQueryHandler(t)
	c, rec := newJSONRequest(t, "/api/notes/renotes", `{"noteId":"ghost"}`)
	require.NoError(t, h.Renotes(c))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRenotes_InvalidParam(t *testing.T) {
	h, _ := newQueryHandler(t)
	c, rec := newJSONRequest(t, "/api/notes/renotes", `{}`)
	require.NoError(t, h.Renotes(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRenotes_InvalidJSON(t *testing.T) {
	h, _ := newQueryHandler(t)
	c, rec := newJSONRequest(t, "/api/notes/renotes", `{invalid`)
	require.NoError(t, h.Renotes(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestReplies_OK(t *testing.T) {
	h, repo := newQueryHandler(t)
	parent := seedPublicNote(repo, "p")
	pid := parent.ID
	r := seedPublicNote(repo, "r")
	r.ReplyID = &pid

	c, rec := newJSONRequest(t, "/api/notes/replies", `{"noteId":"p"}`)
	require.NoError(t, h.Replies(c))
	require.Equal(t, http.StatusOK, rec.Code)
	var resp []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp, 1)
}

func TestReplies_NotFound(t *testing.T) {
	h, _ := newQueryHandler(t)
	c, rec := newJSONRequest(t, "/api/notes/replies", `{"noteId":"ghost"}`)
	require.NoError(t, h.Replies(c))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestChildren_OK(t *testing.T) {
	h, repo := newQueryHandler(t)
	parent := seedPublicNote(repo, "p")
	pid := parent.ID
	r := seedPublicNote(repo, "child1")
	r.ReplyID = &pid
	q := seedPublicNote(repo, "child2")
	q.RenoteID = &pid

	c, rec := newJSONRequest(t, "/api/notes/children", `{"noteId":"p","limit":50}`)
	require.NoError(t, h.Children(c))
	require.Equal(t, http.StatusOK, rec.Code)
	var resp []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp, 2)
}

func TestChildren_LimitClamping(t *testing.T) {
	h, repo := newQueryHandler(t)
	seedPublicNote(repo, "p")
	c, rec := newJSONRequest(t, "/api/notes/children", `{"noteId":"p","limit":1000}`)
	require.NoError(t, h.Children(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestSearch_OK(t *testing.T) {
	h, repo := newQueryHandler(t)
	hello := "Hello world"
	n := seedPublicNote(repo, "n1")
	n.Text = &hello

	c, rec := newJSONRequest(t, "/api/notes/search", `{"query":"hello"}`)
	require.NoError(t, h.Search(c))
	require.Equal(t, http.StatusOK, rec.Code)
	var resp []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp, 1)
}

func TestSearch_LimitClamping(t *testing.T) {
	h, _ := newQueryHandler(t)
	c, rec := newJSONRequest(t, "/api/notes/search", `{"query":"x","limit":1000}`)
	require.NoError(t, h.Search(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestSearch_EmptyQuery(t *testing.T) {
	h, _ := newQueryHandler(t)
	c, rec := newJSONRequest(t, "/api/notes/search", `{"query":""}`)
	require.NoError(t, h.Search(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSearch_InvalidJSON(t *testing.T) {
	h, _ := newQueryHandler(t)
	c, rec := newJSONRequest(t, "/api/notes/search", `{invalid`)
	require.NoError(t, h.Search(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestSearch_NoSearchService verifies the early-out branch when search is
// not configured at all (e.g. test handlers built without injecting one).
func TestSearch_NoSearchService(t *testing.T) {
	noteRepo := testutil.NewMockNoteRepository()
	pollRepo := testutil.NewMockPollRepository()
	idGen, _ := id.NewGenerator("aidx")
	createSvc := corenote.NewCreateService(noteRepo, pollRepo, idGen, nil)
	deleteSvc := corenote.NewDeleteService(noteRepo)
	querySvc := corenote.NewQueryService(noteRepo, nil)
	h := NewHandler(noteRepo, createSvc, deleteSvc, querySvc, nil, nil, nil, nil, idGen)

	c, rec := newJSONRequest(t, "/api/notes/search", `{"query":"hello"}`)
	require.NoError(t, h.Search(c))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// TestSearch_DateCursorsConvertToIDs exercises the sinceDate / untilDate
// fallback path that runs the id generator.
func TestSearch_DateCursorsConvertToIDs(t *testing.T) {
	h, repo := newQueryHandler(t)
	hello := "Hello world"
	n := seedPublicNote(repo, "n1")
	n.Text = &hello

	body := `{"query":"hello","sinceDate":1700000000000,"untilDate":1900000000000}`
	c, rec := newJSONRequest(t, "/api/notes/search", body)
	require.NoError(t, h.Search(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestSearch_RichFilters covers the userId / channelId / host fields by
// confirming a search with extra opts still completes successfully.
func TestSearch_RichFilters(t *testing.T) {
	h, repo := newQueryHandler(t)
	hello := "Hello"
	n := seedPublicNote(repo, "n1")
	n.Text = &hello
	n.UserID = "u1"
	body := `{"query":"hello","userId":"u1","channelId":"","host":"."}`
	c, rec := newJSONRequest(t, "/api/notes/search", body)
	require.NoError(t, h.Search(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestState_OK(t *testing.T) {
	h, repo := newQueryHandler(t)
	seedPublicNote(repo, "n1")

	c, rec := newJSONRequest(t, "/api/notes/state", `{"noteId":"n1"}`)
	require.NoError(t, h.State(c))
	require.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]bool
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp["isFavorited"])
}

func TestState_NotFound(t *testing.T) {
	h, _ := newQueryHandler(t)
	c, rec := newJSONRequest(t, "/api/notes/state", `{"noteId":"ghost"}`)
	require.NoError(t, h.State(c))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestState_InvalidParam(t *testing.T) {
	h, _ := newQueryHandler(t)
	c, rec := newJSONRequest(t, "/api/notes/state", `{}`)
	require.NoError(t, h.State(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestConversation_OK(t *testing.T) {
	h, repo := newQueryHandler(t)
	root := seedPublicNote(repo, "root")
	rid := root.ID
	leaf := seedPublicNote(repo, "leaf")
	leaf.ReplyID = &rid

	c, rec := newJSONRequest(t, "/api/notes/conversation", `{"noteId":"leaf"}`)
	require.NoError(t, h.Conversation(c))
	require.Equal(t, http.StatusOK, rec.Code)
	var resp []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp, 1)
	assert.Equal(t, "root", resp[0]["id"])
}

func TestConversation_LimitClamping(t *testing.T) {
	h, repo := newQueryHandler(t)
	seedPublicNote(repo, "n1")
	c, rec := newJSONRequest(t, "/api/notes/conversation", `{"noteId":"n1","limit":1000}`)
	require.NoError(t, h.Conversation(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestConversation_NotFound(t *testing.T) {
	h, _ := newQueryHandler(t)
	c, rec := newJSONRequest(t, "/api/notes/conversation", `{"noteId":"ghost"}`)
	require.NoError(t, h.Conversation(c))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestConversation_InvalidParam(t *testing.T) {
	h, _ := newQueryHandler(t)
	c, rec := newJSONRequest(t, "/api/notes/conversation", `{}`)
	require.NoError(t, h.Conversation(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// failingQueryRepo causes ListRenotesOf to fail to verify the internalError path.
type failingQueryRepo struct {
	*testutil.MockNoteRepository
}

func (f *failingQueryRepo) ListRenotesOf(_, _, _ string, _ int) ([]*model.Note, error) {
	return nil, testutil.ErrNotFound
}
func (f *failingQueryRepo) ListRepliesOf(_, _, _ string, _ int) ([]*model.Note, error) {
	return nil, testutil.ErrNotFound
}
func (f *failingQueryRepo) ListChildrenOf(_, _, _ string, _ int) ([]*model.Note, error) {
	return nil, testutil.ErrNotFound
}
func (f *failingQueryRepo) SearchByFilter(_ model.NoteSearchFilter) ([]*model.Note, error) {
	return nil, testutil.ErrNotFound
}

func newFailingQueryHandler(t *testing.T) *Handler {
	t.Helper()
	mock := testutil.NewMockNoteRepository()
	seedPublicNote(mock, "p")
	repo := &failingQueryRepo{MockNoteRepository: mock}
	pollRepo := testutil.NewMockPollRepository()
	idGen, _ := id.NewGenerator("aidx")
	createSvc := corenote.NewCreateService(repo, pollRepo, idGen, nil)
	deleteSvc := corenote.NewDeleteService(repo)
	querySvc := corenote.NewQueryService(repo, nil)
	searchSvc := search.NewService(search.NewSQLLikeProvider(repo, nil))
	return NewHandler(repo, createSvc, deleteSvc, querySvc, nil, nil, nil, searchSvc, idGen)
}

func TestRenotes_RepoError(t *testing.T) {
	h := newFailingQueryHandler(t)
	c, rec := newJSONRequest(t, "/api/notes/renotes", `{"noteId":"p"}`)
	require.NoError(t, h.Renotes(c))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestReplies_RepoError(t *testing.T) {
	h := newFailingQueryHandler(t)
	c, rec := newJSONRequest(t, "/api/notes/replies", `{"noteId":"p"}`)
	require.NoError(t, h.Replies(c))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestChildren_RepoError(t *testing.T) {
	h := newFailingQueryHandler(t)
	c, rec := newJSONRequest(t, "/api/notes/children", `{"noteId":"p"}`)
	require.NoError(t, h.Children(c))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestSearch_RepoError(t *testing.T) {
	h := newFailingQueryHandler(t)
	c, rec := newJSONRequest(t, "/api/notes/search", `{"query":"x"}`)
	require.NoError(t, h.Search(c))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// findFailQueryRepo causes FindByIDWithUser to fail so that State/Conversation
// triggerするServiceエラー (Show経由) でnoSuchNoteパスが踏まれる。
type findFailQueryRepo struct {
	*testutil.MockNoteRepository
}

func (f *findFailQueryRepo) FindByIDWithUser(_ string) (*model.Note, error) {
	return nil, testutil.ErrNotFound
}

// TestCreate_ReplyTargetNotFound triggers the new "No such note" branch in Create
// when the reply target does not exist.
func TestCreate_ReplyTargetNotFound(t *testing.T) {
	h, _ := newQueryHandler(t)
	user := &model.User{ID: "u", Username: "u"}

	body := `{"text":"hi","replyId":"ghost"}`
	c, rec := newJSONRequest(t, "/api/notes/create", body)
	setAuthUser(c, user)
	require.NoError(t, h.Create(c))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// TestCreate_RenoteTargetInvisible triggers the new "forbidden" branch when the
// renote target is not visible to the actor.
func TestCreate_RenoteTargetInvisible(t *testing.T) {
	h, repo := newQueryHandler(t)
	repo.Notes["secret"] = &model.Note{
		ID: "secret", UserID: "author", Visibility: model.NoteVisibilityFollowers,
	}
	user := &model.User{ID: "viewer", Username: "viewer"}

	body := `{"renoteId":"secret"}`
	c, rec := newJSONRequest(t, "/api/notes/create", body)
	setAuthUser(c, user)
	require.NoError(t, h.Create(c))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// TestShow_FallbackNoQueryService verifies the lookupVisible nil-queryService path.
func TestShow_FallbackNoQueryService(t *testing.T) {
	noteRepo := testutil.NewMockNoteRepository()
	pollRepo := testutil.NewMockPollRepository()
	idGen, _ := id.NewGenerator("aidx")
	createSvc := corenote.NewCreateService(noteRepo, pollRepo, idGen, nil)
	deleteSvc := corenote.NewDeleteService(noteRepo)
	// queryServiceなしで初期化することでフォールバック経路を取る
	h := NewHandler(noteRepo, createSvc, deleteSvc, nil, nil, nil, nil, nil, idGen)

	seedPublicNote(noteRepo, "n1")
	c, rec := newJSONRequest(t, "/api/notes/show", `{"noteId":"n1"}`)
	require.NoError(t, h.Show(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}
