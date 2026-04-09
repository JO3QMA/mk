package hashtags_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/hashtags"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

var testDB *gorm.DB

func init() {
	testDB = testutil.MustOpenTestDB()
	testutil.ApplyMigrations(testDB)
}

func newHandler() *hashtags.Handler {
	return hashtags.NewHandler(testDB)
}

func brokenHandler() *hashtags.Handler {
	db := testutil.MustOpenTestDB()
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()
	return hashtags.NewHandler(db)
}

func doPost(h func(echo.Context) error, body string) *httptest.ResponseRecorder {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	_ = h(c)
	return rec
}

func cleanup() {
	testDB.Exec(`DELETE FROM "hashtag"`)
}

// --- List ---

func TestList_Success(t *testing.T) {
	cleanup()
	assert.Equal(t, http.StatusOK, doPost(newHandler().List, `{}`).Code)
}

func TestList_WithData(t *testing.T) {
	cleanup()
	testDB.Create(&model.Hashtag{ID: "ht1", Name: "golang", MentionedUsersCount: 5})
	defer cleanup()
	rec := doPost(newHandler().List, `{"limit":10}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "golang")
}

func TestList_WithOffset(t *testing.T) {
	assert.Equal(t, http.StatusOK, doPost(newHandler().List, `{"limit":5,"offset":1}`).Code)
}

func TestList_LimitCap(t *testing.T) {
	assert.Equal(t, http.StatusOK, doPost(newHandler().List, `{"limit":999}`).Code)
}

func TestList_InvalidJSON(t *testing.T) {
	assert.Equal(t, http.StatusBadRequest, doPost(newHandler().List, `invalid`).Code)
}

func TestList_DBError(t *testing.T) {
	assert.Equal(t, http.StatusInternalServerError, doPost(brokenHandler().List, `{}`).Code)
}

// --- Search ---

func TestSearch_Success(t *testing.T) {
	cleanup()
	testDB.Create(&model.Hashtag{ID: "ht2", Name: "gotest"})
	defer cleanup()
	rec := doPost(newHandler().Search, `{"query":"gotest"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "gotest")
}

func TestSearch_InvalidParam(t *testing.T) {
	assert.Equal(t, http.StatusBadRequest, doPost(newHandler().Search, `{}`).Code)
}

func TestSearch_DBError(t *testing.T) {
	assert.Equal(t, http.StatusInternalServerError, doPost(brokenHandler().Search, `{"query":"x"}`).Code)
}

// --- Show ---

func TestShow_Found(t *testing.T) {
	cleanup()
	testDB.Create(&model.Hashtag{ID: "ht3", Name: "rustlang"})
	defer cleanup()
	rec := doPost(newHandler().Show, `{"tag":"rustlang"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "rustlang")
}

func TestShow_NotFound(t *testing.T) {
	assert.Equal(t, http.StatusNotFound, doPost(newHandler().Show, `{"tag":"ghost"}`).Code)
}

func TestShow_InvalidParam(t *testing.T) {
	assert.Equal(t, http.StatusBadRequest, doPost(newHandler().Show, `{}`).Code)
}

// --- Trend ---

func TestTrend_Success(t *testing.T) {
	cleanup()
	testDB.Create(&model.Hashtag{ID: "ht4", Name: "trending", MentionedUsersCount: 100})
	defer cleanup()
	rec := doPost(newHandler().Trend, `{}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "trending")
}

func TestTrend_DBError(t *testing.T) {
	assert.Equal(t, http.StatusInternalServerError, doPost(brokenHandler().Trend, `{}`).Code)
}

// --- Users ---

func TestUsers_Success(t *testing.T) {
	assert.Equal(t, http.StatusOK, doPost(newHandler().Users, `{"tag":"test"}`).Code)
}

func TestUsers_InvalidParam(t *testing.T) {
	assert.Equal(t, http.StatusBadRequest, doPost(newHandler().Users, `{}`).Code)
}
