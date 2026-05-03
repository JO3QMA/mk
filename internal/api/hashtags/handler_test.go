package hashtags_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
	"github.com/shiroha-a/mk/internal/api/hashtags"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// cleanupNotes は Trend 関連 test の前後で note と user (FK) を掃除する。
func cleanupNotes() {
	testDB.Exec(`DELETE FROM "note"`)
	testDB.Exec(`DELETE FROM "user" WHERE "id" LIKE 'tu_%'`)
}

// ensureUser は note の userId FK を満たすための最小限の user row を
// 用意する。同 id が既存の場合は no-op。
func ensureUser(t *testing.T, userID string) {
	t.Helper()
	testDB.Exec(`
		INSERT INTO "user" ("id", "username", "usernameLower")
		VALUES (?, ?, ?)
		ON CONFLICT DO NOTHING
	`, userID, userID, userID)
}

// insertNote: Trend test 用の最小限 note row を作る。aidx ID を
// createdAt として埋め込み、note.tags に集計対象タグを入れる。
// model.Note 経由だと validation が重いので raw SQL で軽く済ます。
func insertNote(t *testing.T, gen id.Generator, userID string, tags []string, createdAt time.Time) {
	t.Helper()
	ensureUser(t, userID)
	noteID := gen.Generate(createdAt)
	require.NoError(t, testDB.Exec(
		`INSERT INTO "note" ("id", "userId", "tags", "visibility", "text")
		 VALUES (?, ?, ?, 'public', '')`,
		noteID, userID, pq.StringArray(tags),
	).Error)
}

func TestTrend_EmptyReturnsEmptyArray(t *testing.T) {
	cleanupNotes()
	rec := doPost(newHandler().Trend, `{}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	var resp []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Empty(t, resp, "no notes → empty trend list")
}

func TestTrend_AggregatesRecentNotes(t *testing.T) {
	cleanupNotes()
	defer cleanupNotes()

	gen, err := id.NewGenerator("aidx")
	require.NoError(t, err)

	now := time.Now()
	// alpha: 2 distinct users (tu_1@bucket0, tu_2@bucket1)
	insertNote(t, gen, "tu_1", []string{"alpha"}, now.Add(-5*time.Minute))
	insertNote(t, gen, "tu_2", []string{"alpha"}, now.Add(-15*time.Minute))
	// beta: 1 user (tu_1@bucket0)
	insertNote(t, gen, "tu_1", []string{"beta"}, now.Add(-5*time.Minute))
	// gamma: 1 note だが 200 分超 → bucket 範囲外、cutoff で除外
	insertNote(t, gen, "tu_3", []string{"gamma"}, now.Add(-300*time.Minute))

	rec := doPost(newHandler().Trend, `{}`)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	// alpha (total 2 users) → beta (total 1) の順にランクされる。gamma は除外。
	require.Len(t, resp, 2, "alpha + beta only")
	assert.Equal(t, "alpha", resp[0]["tag"])
	assert.Equal(t, "beta", resp[1]["tag"])

	// chart 配列は 20 buckets 固定
	chart := resp[0]["chart"].([]any)
	require.Len(t, chart, 20, "chart bucket count = 20")

	// alpha: bucket 0 (直近 10 分) と bucket 1 (10-20 分) に各 1 user
	assert.EqualValues(t, 1, chart[0], "alpha bucket 0 has tu_1")
	assert.EqualValues(t, 1, chart[1], "alpha bucket 1 has tu_2")
	for i := 2; i < 20; i++ {
		assert.EqualValues(t, 0, chart[i], "alpha bucket %d empty", i)
	}

	// usersCount は max(chart) で alpha は 1 (各 bucket の users が 1)
	assert.EqualValues(t, 1, resp[0]["usersCount"])
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
