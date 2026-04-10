package notes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/server/middleware"
	"github.com/stretchr/testify/assert"
)

func postDraft(handler func(echo.Context) error, body string, user *model.User) *httptest.ResponseRecorder {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if user != nil {
		c.Set(string(middleware.UserContextKey), user)
	}
	_ = handler(c)
	return rec
}

func newDraftHandler() *Handler {
	idGen, _ := id.NewGenerator("aidx")
	h := &Handler{idGen: idGen}
	return h
}

// --- DraftsList ---

func TestDraftsList_NilDB(t *testing.T) {
	h := newDraftHandler()
	rec := postDraft(h.DraftsList, `{}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "[]\n", rec.Body.String())
}

// --- DraftsCreate ---

func TestDraftsCreate_NilDB(t *testing.T) {
	h := newDraftHandler()
	rec := postDraft(h.DraftsCreate, `{"text":"hello"}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestDraftsCreate_NilDB_InvalidJSON(t *testing.T) {
	h := newDraftHandler()
	// draftDB nil → NoContent (Bindの前にreturn)
	rec := postDraft(h.DraftsCreate, `invalid`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

// --- DraftsUpdate ---

func TestDraftsUpdate_NilDB(t *testing.T) {
	h := newDraftHandler()
	rec := postDraft(h.DraftsUpdate, `{"draftId":"d1"}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestDraftsUpdate_NilDB_InvalidParam(t *testing.T) {
	h := newDraftHandler()
	rec := postDraft(h.DraftsUpdate, `{}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

// --- DraftsDelete ---

func TestDraftsDelete_NilDB(t *testing.T) {
	h := newDraftHandler()
	rec := postDraft(h.DraftsDelete, `{"draftId":"d1"}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestDraftsDelete_NilDB_InvalidParam(t *testing.T) {
	h := newDraftHandler()
	// draftDB nil → NoContent (パラメータチェック前にreturn)
	rec := postDraft(h.DraftsDelete, `{}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

// --- DraftsCount ---

func TestDraftsCount_NilDB(t *testing.T) {
	h := newDraftHandler()
	rec := postDraft(h.DraftsCount, `{}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"count":0`)
}

// --- ThreadMuting ---

func TestThreadMutingCreate_Success(t *testing.T) {
	h := newDraftHandler()
	rec := postDraft(h.ThreadMutingCreate, `{"noteId":"n1"}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestThreadMutingCreate_InvalidParam(t *testing.T) {
	h := newDraftHandler()
	rec := postDraft(h.ThreadMutingCreate, `{}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestThreadMutingDelete_Success(t *testing.T) {
	h := newDraftHandler()
	rec := postDraft(h.ThreadMutingDelete, `{"noteId":"n1"}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestThreadMutingDelete_InvalidParam(t *testing.T) {
	h := newDraftHandler()
	rec := postDraft(h.ThreadMutingDelete, `{}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- PollsRecommendation ---

func TestPollsRecommendation(t *testing.T) {
	h := newDraftHandler()
	rec := postDraft(h.PollsRecommendation, `{}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "[]\n", rec.Body.String())
}
