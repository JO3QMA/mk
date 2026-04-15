package admin_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- SystemWebhook -----------------------------------------------------------

func TestSystemWebhookCreate_Success(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	repo := testutil.NewMockSystemWebhookRepository()
	h.SetSystemWebhookRepo(repo)

	rec := doPost(h.SystemWebhookCreate,
		`{"name":"hook1","url":"https://example.com/hook","secret":"s","on":["abuseReport"]}`,
		adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)

	var got model.SystemWebhook
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "hook1", got.Name)
	assert.Equal(t, "https://example.com/hook", got.URL)
	assert.True(t, got.IsActive)
	assert.Contains(t, repo.Webhooks, got.ID)
}

func TestSystemWebhookCreate_MissingFields(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetSystemWebhookRepo(testutil.NewMockSystemWebhookRepository())
	rec := doPost(h.SystemWebhookCreate, `{"name":""}`, adminUser)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSystemWebhookCreate_RepoError(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	repo := testutil.NewMockSystemWebhookRepository()
	repo.CreateErr = assertError{}
	h.SetSystemWebhookRepo(repo)
	rec := doPost(h.SystemWebhookCreate, `{"name":"x","url":"https://x"}`, adminUser)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestSystemWebhookList_ReturnsRows(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	repo := testutil.NewMockSystemWebhookRepository()
	require.NoError(t, repo.Create(&model.SystemWebhook{ID: "w1", Name: "a", URL: "u", IsActive: true}))
	require.NoError(t, repo.Create(&model.SystemWebhook{ID: "w2", Name: "b", URL: "u", IsActive: true}))
	h.SetSystemWebhookRepo(repo)

	rec := doPost(h.SystemWebhookList, `{}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var rows []model.SystemWebhook
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rows))
	assert.Len(t, rows, 2)
	// 並び順は id DESC
	assert.Equal(t, "w2", rows[0].ID)
	assert.Equal(t, "w1", rows[1].ID)
}

func TestSystemWebhookShow_FoundAndNotFound(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	repo := testutil.NewMockSystemWebhookRepository()
	require.NoError(t, repo.Create(&model.SystemWebhook{ID: "w1", Name: "hook"}))
	h.SetSystemWebhookRepo(repo)

	rec := doPost(h.SystemWebhookShow, `{"id":"w1"}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)

	rec2 := doPost(h.SystemWebhookShow, `{"id":"missing"}`, adminUser)
	assert.Equal(t, http.StatusNotFound, rec2.Code)
}

func TestSystemWebhookDelete_Removes(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	repo := testutil.NewMockSystemWebhookRepository()
	require.NoError(t, repo.Create(&model.SystemWebhook{ID: "w1"}))
	h.SetSystemWebhookRepo(repo)

	rec := doPost(h.SystemWebhookDelete, `{"id":"w1"}`, adminUser)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.NotContains(t, repo.Webhooks, "w1")
}

func TestSystemWebhookUpdate_PartialFields(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	repo := testutil.NewMockSystemWebhookRepository()
	require.NoError(t, repo.Create(&model.SystemWebhook{
		ID: "w1", Name: "old", URL: "https://old", IsActive: true,
	}))
	h.SetSystemWebhookRepo(repo)

	rec := doPost(h.SystemWebhookUpdate,
		`{"id":"w1","name":"new","isActive":false}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "new", repo.Webhooks["w1"].Name)
	assert.Equal(t, "https://old", repo.Webhooks["w1"].URL)
	assert.False(t, repo.Webhooks["w1"].IsActive)
}

func TestSystemWebhookUpdate_NotFound(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetSystemWebhookRepo(testutil.NewMockSystemWebhookRepository())
	rec := doPost(h.SystemWebhookUpdate, `{"id":"missing","name":"x"}`, adminUser)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestSystemWebhookUpdate_MissingID(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetSystemWebhookRepo(testutil.NewMockSystemWebhookRepository())
	rec := doPost(h.SystemWebhookUpdate, `{}`, adminUser)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSystemWebhookTest_WithRepo(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	repo := testutil.NewMockSystemWebhookRepository()
	require.NoError(t, repo.Create(&model.SystemWebhook{ID: "w1", URL: "https://127.0.0.1:1"}))
	h.SetSystemWebhookRepo(repo)

	// webhookId 空は 204
	assert.Equal(t, http.StatusNoContent, doPost(h.SystemWebhookTest, `{}`, adminUser).Code)
	// 存在しない webhookId も 204
	assert.Equal(t, http.StatusNoContent,
		doPost(h.SystemWebhookTest, `{"webhookId":"missing"}`, adminUser).Code)
	// 正常系も 204 (fire-and-forget)
	assert.Equal(t, http.StatusNoContent,
		doPost(h.SystemWebhookTest, `{"webhookId":"w1","type":"abuseReport"}`, adminUser).Code)
}

// --- AbuseReportNotificationRecipient ---------------------------------------

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
