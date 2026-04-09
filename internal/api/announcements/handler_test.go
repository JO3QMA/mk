package announcements_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/announcements"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/server/middleware"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestHandler(t *testing.T) (*announcements.Handler, *testutil.MockAnnouncementRepository) {
	t.Helper()
	repo := testutil.NewMockAnnouncementRepository()
	idGen, _ := id.NewGenerator("aidx")
	return announcements.NewHandler(repo, idGen), repo
}

func doPost(h func(echo.Context) error, body string, user *model.User) *httptest.ResponseRecorder {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if user != nil {
		c.Set(string(middleware.UserContextKey), user)
	}
	_ = h(c)
	return rec
}

// --- Public ---

func TestList_Empty(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doPost(h.List, `{}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestList_WithItems(t *testing.T) {
	h, repo := newTestHandler(t)
	// 有効なAIDX IDを使ってcreatedAt解析パスをカバー
	idGen, _ := id.NewGenerator("aidx")
	validID := idGen.Generate(java_time())
	repo.Items[validID] = &model.Announcement{ID: validID, Title: "Hi", Text: "Hello", IsActive: true}
	rec := doPost(h.List, `{}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusOK, rec.Code)
	var resp []any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp, 1)
	item := resp[0].(map[string]any)
	assert.NotEmpty(t, item["createdAt"])
}

func java_time() time.Time {
	return time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
}

func TestList_InvalidJSON(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doPost(h.List, `invalid`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestList_ActiveFalse(t *testing.T) {
	h, repo := newTestHandler(t)
	repo.Items["a1"] = &model.Announcement{ID: "a1", IsActive: false}
	f := false
	_ = f
	rec := doPost(h.List, `{"isActive":false}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- ReadAnnouncement ---

func TestReadAnnouncement_Success(t *testing.T) {
	h, repo := newTestHandler(t)
	repo.Items["a1"] = &model.Announcement{ID: "a1", IsActive: true}
	rec := doPost(h.ReadAnnouncement, `{"announcementId":"a1"}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestReadAnnouncement_AlreadyRead(t *testing.T) {
	h, repo := newTestHandler(t)
	repo.Items["a1"] = &model.Announcement{ID: "a1"}
	doPost(h.ReadAnnouncement, `{"announcementId":"a1"}`, &model.User{ID: "u1"})
	rec := doPost(h.ReadAnnouncement, `{"announcementId":"a1"}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestReadAnnouncement_NotFound(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doPost(h.ReadAnnouncement, `{"announcementId":"ghost"}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestReadAnnouncement_InvalidParam(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doPost(h.ReadAnnouncement, `{}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Admin ---

func TestAdminCreate_Success(t *testing.T) {
	h, repo := newTestHandler(t)
	rec := doPost(h.AdminCreate, `{"title":"News","text":"Big news!"}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Len(t, repo.Items, 1)
}

func TestAdminCreate_InvalidParam(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doPost(h.AdminCreate, `{}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdminUpdate_Success(t *testing.T) {
	h, repo := newTestHandler(t)
	repo.Items["a1"] = &model.Announcement{ID: "a1", Title: "Old"}
	rec := doPost(h.AdminUpdate, `{"id":"a1","title":"New"}`, nil)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestAdminUpdate_AllFields(t *testing.T) {
	h, repo := newTestHandler(t)
	repo.Items["a1"] = &model.Announcement{ID: "a1", Title: "Old", IsActive: true}
	rec := doPost(h.AdminUpdate, `{"id":"a1","title":"New","text":"txt","isActive":false}`, nil)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestAdminUpdate_InvalidParam(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doPost(h.AdminUpdate, `{}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdminDelete_Success(t *testing.T) {
	h, repo := newTestHandler(t)
	repo.Items["a1"] = &model.Announcement{ID: "a1"}
	rec := doPost(h.AdminDelete, `{"id":"a1"}`, nil)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestAdminDelete_InvalidParam(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doPost(h.AdminDelete, `{}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdminList_Success(t *testing.T) {
	h, repo := newTestHandler(t)
	repo.Items["a1"] = &model.Announcement{ID: "a1"}
	rec := doPost(h.AdminList, `{}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAdminList_InvalidJSON(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doPost(h.AdminList, `invalid`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Failing repo tests ---

type failingListAnnouncementRepo struct {
	*testutil.MockAnnouncementRepository
}

func (f *failingListAnnouncementRepo) List(_ bool, _, _ int) ([]*model.Announcement, error) {
	return nil, assert.AnError
}

func TestList_Error(t *testing.T) {
	idGen, _ := id.NewGenerator("aidx")
	h := announcements.NewHandler(&failingListAnnouncementRepo{testutil.NewMockAnnouncementRepository()}, idGen)
	rec := doPost(h.List, `{}`, nil)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestAdminList_Error(t *testing.T) {
	idGen, _ := id.NewGenerator("aidx")
	h := announcements.NewHandler(&failingListAnnouncementRepo{testutil.NewMockAnnouncementRepository()}, idGen)
	rec := doPost(h.AdminList, `{}`, nil)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

type failingCreateAnnouncementRepo struct {
	*testutil.MockAnnouncementRepository
}

func (f *failingCreateAnnouncementRepo) Create(_ *model.Announcement) error { return assert.AnError }

func TestAdminCreate_Error(t *testing.T) {
	idGen, _ := id.NewGenerator("aidx")
	h := announcements.NewHandler(&failingCreateAnnouncementRepo{testutil.NewMockAnnouncementRepository()}, idGen)
	rec := doPost(h.AdminCreate, `{"title":"x","text":"y"}`, nil)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

type failingMarkReadRepo struct {
	*testutil.MockAnnouncementRepository
}

func (f *failingMarkReadRepo) MarkRead(_ *model.AnnouncementRead) error { return assert.AnError }

type failingUpdateAnnouncementRepo struct {
	*testutil.MockAnnouncementRepository
}

func (f *failingUpdateAnnouncementRepo) UpdateFields(_ string, _ map[string]any) error {
	return assert.AnError
}

func TestAdminUpdate_Error(t *testing.T) {
	idGen, _ := id.NewGenerator("aidx")
	h := announcements.NewHandler(&failingUpdateAnnouncementRepo{testutil.NewMockAnnouncementRepository()}, idGen)
	rec := doPost(h.AdminUpdate, `{"id":"x","title":"y"}`, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

type failingDeleteAnnouncementRepo struct {
	*testutil.MockAnnouncementRepository
}

func (f *failingDeleteAnnouncementRepo) Delete(_ string) error { return assert.AnError }

func TestAdminDelete_Error(t *testing.T) {
	idGen, _ := id.NewGenerator("aidx")
	h := announcements.NewHandler(&failingDeleteAnnouncementRepo{testutil.NewMockAnnouncementRepository()}, idGen)
	rec := doPost(h.AdminDelete, `{"id":"x"}`, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestReadAnnouncement_MarkReadError(t *testing.T) {
	repo := &failingMarkReadRepo{testutil.NewMockAnnouncementRepository()}
	repo.Items["a1"] = &model.Announcement{ID: "a1"}
	idGen, _ := id.NewGenerator("aidx")
	h := announcements.NewHandler(repo, idGen)
	rec := doPost(h.ReadAnnouncement, `{"announcementId":"a1"}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
