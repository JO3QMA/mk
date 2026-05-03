package admin_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- AbuseReportNotificationRecipient ---

func TestRecipientCreate_Success(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	repo := testutil.NewMockAbuseReportNotificationRecipientRepository()
	h.SetRecipientRepo(repo)

	rec := doPost(h.AbuseReportNotificationRecipientCreate,
		`{"name":"ops","method":"email"}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)

	var got model.AbuseReportNotificationRecipient
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "ops", got.Name)
	assert.Equal(t, "email", got.Method)
	assert.True(t, got.IsActive)
}

func TestRecipientCreate_DefaultMethod(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetRecipientRepo(testutil.NewMockAbuseReportNotificationRecipientRepository())
	rec := doPost(h.AbuseReportNotificationRecipientCreate, `{"name":"ops"}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var got model.AbuseReportNotificationRecipient
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "email", got.Method)
}

func TestRecipientCreate_RepoError(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	repo := testutil.NewMockAbuseReportNotificationRecipientRepository()
	repo.CreateErr = assertError{}
	h.SetRecipientRepo(repo)
	rec := doPost(h.AbuseReportNotificationRecipientCreate, `{"name":"x"}`, adminUser)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestRecipientList_ReturnsRows(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	repo := testutil.NewMockAbuseReportNotificationRecipientRepository()
	require.NoError(t, repo.Create(&model.AbuseReportNotificationRecipient{ID: "r1", Name: "a", Method: "email"}))
	require.NoError(t, repo.Create(&model.AbuseReportNotificationRecipient{ID: "r2", Name: "b", Method: "webhook"}))
	h.SetRecipientRepo(repo)

	rec := doPost(h.AbuseReportNotificationRecipientList, `{}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var rows []model.AbuseReportNotificationRecipient
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rows))
	assert.Len(t, rows, 2)
}

func TestRecipientShow_FoundAndNotFound(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	repo := testutil.NewMockAbuseReportNotificationRecipientRepository()
	require.NoError(t, repo.Create(&model.AbuseReportNotificationRecipient{ID: "r1", Name: "ops", Method: "email"}))
	h.SetRecipientRepo(repo)

	rec := doPost(h.AbuseReportNotificationRecipientShow, `{"id":"r1"}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)

	rec2 := doPost(h.AbuseReportNotificationRecipientShow, `{"id":"missing"}`, adminUser)
	assert.Equal(t, http.StatusNotFound, rec2.Code)
}

func TestRecipientDelete_Removes(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	repo := testutil.NewMockAbuseReportNotificationRecipientRepository()
	require.NoError(t, repo.Create(&model.AbuseReportNotificationRecipient{ID: "r1"}))
	h.SetRecipientRepo(repo)

	rec := doPost(h.AbuseReportNotificationRecipientDelete, `{"id":"r1"}`, adminUser)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.NotContains(t, repo.Recipients, "r1")
}

func TestRecipientUpdate_PartialFields(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	repo := testutil.NewMockAbuseReportNotificationRecipientRepository()
	require.NoError(t, repo.Create(&model.AbuseReportNotificationRecipient{
		ID: "r1", Name: "old", Method: "email", IsActive: true,
	}))
	h.SetRecipientRepo(repo)

	rec := doPost(h.AbuseReportNotificationRecipientUpdate,
		`{"id":"r1","name":"new","method":"webhook","isActive":false}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "new", repo.Recipients["r1"].Name)
	assert.Equal(t, "webhook", repo.Recipients["r1"].Method)
	assert.False(t, repo.Recipients["r1"].IsActive)
}

func TestRecipientUpdate_NotFound(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetRecipientRepo(testutil.NewMockAbuseReportNotificationRecipientRepository())
	rec := doPost(h.AbuseReportNotificationRecipientUpdate, `{"id":"missing","name":"x"}`, adminUser)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRecipientUpdate_MissingID(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetRecipientRepo(testutil.NewMockAbuseReportNotificationRecipientRepository())
	rec := doPost(h.AbuseReportNotificationRecipientUpdate, `{}`, adminUser)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- thin nil-repo smoke tests ---
//
// nil recipientRepo 経路 (newTestHandler は wire しない) で expected status を
// 返すことを担保。詳細テストは repo を wire して実挙動を検証するので、本群は
// nil 分岐の coverage 補完。

func TestAbuseReportNotificationRecipientCreate(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.AbuseReportNotificationRecipientCreate, `{}`, adminUser).Code)
}

func TestAbuseReportNotificationRecipientDelete(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.AbuseReportNotificationRecipientDelete, `{}`, adminUser).Code)
}

func TestAbuseReportNotificationRecipientList(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusOK, doPost(h.AbuseReportNotificationRecipientList, `{}`, adminUser).Code)
}

func TestAbuseReportNotificationRecipientShow(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNotFound, doPost(h.AbuseReportNotificationRecipientShow, `{}`, adminUser).Code)
}

func TestAbuseReportNotificationRecipientUpdate(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.AbuseReportNotificationRecipientUpdate, `{}`, adminUser).Code)
}

// --- moderation log assertions (#665) ---

func TestRecipientCreate_WritesModerationLog(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetRecipientRepo(testutil.NewMockAbuseReportNotificationRecipientRepository())
	repo := attachModLog(t, h)

	rec := doPost(h.AbuseReportNotificationRecipientCreate, `{"name":"r","method":"email"}`, adminUser)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Eventually(t, func() bool { return len(repo.Snapshot()) == 1 }, 500*time.Millisecond, 5*time.Millisecond)
	assert.Equal(t, "createAbuseReportNotificationRecipient", repo.Snapshot()[0].Type)
}

func TestRecipientUpdate_WritesModerationLog(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rcpRepo := testutil.NewMockAbuseReportNotificationRecipientRepository()
	require.NoError(t, rcpRepo.Create(&model.AbuseReportNotificationRecipient{ID: "r1", Name: "old", Method: "email"}))
	h.SetRecipientRepo(rcpRepo)
	repo := attachModLog(t, h)

	rec := doPost(h.AbuseReportNotificationRecipientUpdate, `{"id":"r1","name":"new"}`, adminUser)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Eventually(t, func() bool { return len(repo.Snapshot()) == 1 }, 500*time.Millisecond, 5*time.Millisecond)
	logs := repo.Snapshot()
	assert.Equal(t, "updateAbuseReportNotificationRecipient", logs[0].Type)
	var info map[string]any
	require.NoError(t, json.Unmarshal(logs[0].Info, &info))
	require.NotNil(t, info["before"])
	require.NotNil(t, info["after"])
}

func TestRecipientDelete_WritesModerationLog(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rcpRepo := testutil.NewMockAbuseReportNotificationRecipientRepository()
	require.NoError(t, rcpRepo.Create(&model.AbuseReportNotificationRecipient{ID: "r1", Name: "doomed", Method: "email"}))
	h.SetRecipientRepo(rcpRepo)
	repo := attachModLog(t, h)

	rec := doPost(h.AbuseReportNotificationRecipientDelete, `{"id":"r1"}`, adminUser)
	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Eventually(t, func() bool { return len(repo.Snapshot()) == 1 }, 500*time.Millisecond, 5*time.Millisecond)
	assert.Equal(t, "deleteAbuseReportNotificationRecipient", repo.Snapshot()[0].Type)
}

// --- moderation log assertions (#665) ---

func TestAbuseReportNotificationRecipientCreate_WritesModerationLog(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetRecipientRepo(testutil.NewMockAbuseReportNotificationRecipientRepository())
	repo := attachModLog(t, h)

	rec := doPost(h.AbuseReportNotificationRecipientCreate, `{"name":"r1","method":"email"}`, adminUser)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Eventually(t, func() bool { return len(repo.Snapshot()) == 1 }, 500*time.Millisecond, 5*time.Millisecond)
	assert.Equal(t, "createAbuseReportNotificationRecipient", repo.Snapshot()[0].Type)
}

func TestAbuseReportNotificationRecipientUpdate_WritesModerationLog(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rRepo := testutil.NewMockAbuseReportNotificationRecipientRepository()
	require.NoError(t, rRepo.Create(&model.AbuseReportNotificationRecipient{ID: "r1", Name: "old"}))
	h.SetRecipientRepo(rRepo)
	repo := attachModLog(t, h)

	rec := doPost(h.AbuseReportNotificationRecipientUpdate, `{"id":"r1","name":"new"}`, adminUser)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Eventually(t, func() bool { return len(repo.Snapshot()) == 1 }, 500*time.Millisecond, 5*time.Millisecond)
	assert.Equal(t, "updateAbuseReportNotificationRecipient", repo.Snapshot()[0].Type)
}

func TestAbuseReportNotificationRecipientDelete_WritesModerationLog(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rRepo := testutil.NewMockAbuseReportNotificationRecipientRepository()
	require.NoError(t, rRepo.Create(&model.AbuseReportNotificationRecipient{ID: "r1"}))
	h.SetRecipientRepo(rRepo)
	repo := attachModLog(t, h)

	rec := doPost(h.AbuseReportNotificationRecipientDelete, `{"id":"r1"}`, adminUser)
	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Eventually(t, func() bool { return len(repo.Snapshot()) == 1 }, 500*time.Millisecond, 5*time.Millisecond)
	assert.Equal(t, "deleteAbuseReportNotificationRecipient", repo.Snapshot()[0].Type)
}
