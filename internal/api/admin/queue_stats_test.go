package admin_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	apiadmin "github.com/shiroha-a/mk/internal/api/admin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubQueueInspector is a test double for admin.QueueInspector that returns
// configurable canned responses without touching Redis.
type stubQueueInspector struct {
	queues        []string
	info          map[string]*apiadmin.QueueInfoResult
	pending       map[string][]*apiadmin.QueueTaskSummary
	active        map[string][]*apiadmin.QueueTaskSummary
	scheduled     map[string][]*apiadmin.QueueTaskSummary
	retry         map[string][]*apiadmin.QueueTaskSummary
	task          map[string]*apiadmin.QueueTaskSummary
	deleted       []string
	runCalls      []string
	deleteAllHits []string
	queuesErr     error
	runErr        error
	deleteErr     error
}

func (s *stubQueueInspector) Queues() ([]string, error) { return s.queues, s.queuesErr }
func (s *stubQueueInspector) GetQueueInfo(q string) (*apiadmin.QueueInfoResult, error) {
	if info, ok := s.info[q]; ok {
		return info, nil
	}
	return nil, errors.New("not found")
}
func (s *stubQueueInspector) DeleteTask(_, id string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.deleted = append(s.deleted, id)
	return nil
}
func (s *stubQueueInspector) DeleteAllPendingTasks(q string) (int, error) {
	s.deleteAllHits = append(s.deleteAllHits, q)
	return 0, nil
}
func (s *stubQueueInspector) RunTask(_, id string) error {
	if s.runErr != nil {
		return s.runErr
	}
	s.runCalls = append(s.runCalls, id)
	return nil
}
func (s *stubQueueInspector) ListPendingTasks(q string, _, _ int) ([]*apiadmin.QueueTaskSummary, error) {
	return s.pending[q], nil
}
func (s *stubQueueInspector) ListActiveTasks(q string, _, _ int) ([]*apiadmin.QueueTaskSummary, error) {
	return s.active[q], nil
}
func (s *stubQueueInspector) ListScheduledTasks(q string, _, _ int) ([]*apiadmin.QueueTaskSummary, error) {
	return s.scheduled[q], nil
}
func (s *stubQueueInspector) ListRetryTasks(q string, _, _ int) ([]*apiadmin.QueueTaskSummary, error) {
	return s.retry[q], nil
}
func (s *stubQueueInspector) GetTaskInfo(_, id string) (*apiadmin.QueueTaskSummary, error) {
	if t, ok := s.task[id]; ok {
		return t, nil
	}
	return nil, errors.New("not found")
}

// --- Queue --------------------------------------------------------------------

func TestQueueClear_WithInspector(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	insp := &stubQueueInspector{queues: []string{"deliver", "inbox"}}
	h.SetQueueInspector(insp)
	rec := doPost(h.QueueClear, `{}`, adminUser)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.ElementsMatch(t, []string{"deliver", "inbox"}, insp.deleteAllHits)
}

func TestQueueClear_QueuesError(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	insp := &stubQueueInspector{queuesErr: errors.New("redis down")}
	h.SetQueueInspector(insp)
	assert.Equal(t, http.StatusInternalServerError, doPost(h.QueueClear, `{}`, adminUser).Code)
}

func TestQueueJobs_FilterByQueueAndState(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	insp := &stubQueueInspector{
		queues: []string{"deliver"},
		pending: map[string][]*apiadmin.QueueTaskSummary{
			"deliver": {{ID: "p1", Queue: "deliver", Type: "x", State: "pending"}},
		},
		active: map[string][]*apiadmin.QueueTaskSummary{
			"deliver": {{ID: "a1", Queue: "deliver", Type: "x", State: "active"}},
		},
	}
	h.SetQueueInspector(insp)

	rec := doPost(h.QueueJobs, `{"queue":"deliver","state":"active"}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var rows []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rows))
	require.Len(t, rows, 1)
	assert.Equal(t, "a1", rows[0]["id"])
}

func TestQueueJobs_AllQueuesPendingDefault(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	insp := &stubQueueInspector{
		queues: []string{"deliver", "inbox"},
		pending: map[string][]*apiadmin.QueueTaskSummary{
			"deliver": {{ID: "d1", Queue: "deliver"}},
			"inbox":   {{ID: "i1", Queue: "inbox"}},
		},
	}
	h.SetQueueInspector(insp)
	rec := doPost(h.QueueJobs, `{}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var rows []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rows))
	assert.Len(t, rows, 2)
}

func TestQueueShowJob_Found(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	insp := &stubQueueInspector{
		task: map[string]*apiadmin.QueueTaskSummary{
			"tid1": {ID: "tid1", Queue: "deliver", Type: "x", State: "pending"},
		},
	}
	h.SetQueueInspector(insp)

	rec := doPost(h.QueueShowJob, `{"queue":"deliver","id":"tid1"}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "tid1", got["id"])
}

func TestQueueShowJob_NotFoundWithInspector(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	insp := &stubQueueInspector{}
	h.SetQueueInspector(insp)
	rec := doPost(h.QueueShowJob, `{"queue":"deliver","id":"ghost"}`, adminUser)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestQueueRemoveJob_Success(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	insp := &stubQueueInspector{}
	h.SetQueueInspector(insp)
	rec := doPost(h.QueueRemoveJob, `{"queue":"deliver","id":"tid1"}`, adminUser)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, []string{"tid1"}, insp.deleted)
}

func TestQueueRemoveJob_NotFound(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	insp := &stubQueueInspector{deleteErr: errors.New("not found")}
	h.SetQueueInspector(insp)
	rec := doPost(h.QueueRemoveJob, `{"queue":"deliver","id":"x"}`, adminUser)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestQueueRetryJob_Success(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	insp := &stubQueueInspector{}
	h.SetQueueInspector(insp)
	rec := doPost(h.QueueRetryJob, `{"queue":"deliver","id":"tid1"}`, adminUser)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, []string{"tid1"}, insp.runCalls)
}

func TestQueuePromoteJobs_RunsScheduledAndRetry(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	insp := &stubQueueInspector{
		queues: []string{"deliver"},
		scheduled: map[string][]*apiadmin.QueueTaskSummary{
			"deliver": {{ID: "s1"}, {ID: "s2"}},
		},
		retry: map[string][]*apiadmin.QueueTaskSummary{
			"deliver": {{ID: "r1"}},
		},
	}
	h.SetQueueInspector(insp)

	rec := doPost(h.QueuePromoteJobs, `{}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.EqualValues(t, 3, got["promoted"])
	assert.ElementsMatch(t, []string{"s1", "s2", "r1"}, insp.runCalls)
}

func TestQueueQueueStats_WithInspector(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	insp := &stubQueueInspector{
		queues: []string{"deliver"},
		info: map[string]*apiadmin.QueueInfoResult{
			"deliver": {Queue: "deliver", Size: 5, Pending: 3, Active: 1, Completed: 10, Failed: 2, Scheduled: 0, Retry: 1},
		},
	}
	h.SetQueueInspector(insp)
	rec := doPost(h.QueueQueueStats, `{}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	deliver, ok := got["deliver"].(map[string]any)
	require.True(t, ok)
	assert.EqualValues(t, 5, deliver["size"])
	assert.EqualValues(t, 3, deliver["pending"])
}

func TestQueueDeliverDelayed_CombinesScheduledAndRetry(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	insp := &stubQueueInspector{
		scheduled: map[string][]*apiadmin.QueueTaskSummary{
			"deliver": {{ID: "s1"}},
		},
		retry: map[string][]*apiadmin.QueueTaskSummary{
			"deliver": {{ID: "r1"}},
		},
	}
	h.SetQueueInspector(insp)
	rec := doPost(h.QueueDeliverDelayed, `{}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var rows []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rows))
	assert.Len(t, rows, 2)
}

// --- Captcha ------------------------------------------------------------------

func TestCaptchaCurrent_WithHcaptchaEnabled(t *testing.T) {
	h, _, metaRepo, _ := newTestHandler(t)
	siteKey := "sk-hcap"
	metaRepo.Meta.EnableHcaptcha = true
	metaRepo.Meta.HcaptchaSiteKey = &siteKey
	rec := doPost(h.CaptchaCurrent, `{}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "hcaptcha", got["provider"])
	assert.Equal(t, "sk-hcap", got["siteKey"])
}

func TestCaptchaCurrent_NoneEnabled(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.CaptchaCurrent, `{}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Nil(t, got["provider"])
}

func TestCaptchaSave_AppliesFlags(t *testing.T) {
	h, _, metaRepo, _ := newTestHandler(t)
	rec := doPost(h.CaptchaSave,
		`{"provider":"turnstile","turnstileSiteKey":"sk","turnstileSecretKey":"ss"}`, adminUser)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.True(t, metaRepo.Meta.EnableTurnstile)
	assert.False(t, metaRepo.Meta.EnableHcaptcha)
}

func TestCaptchaSave_UnknownProvider(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.CaptchaSave, `{"provider":"garbage"}`, adminUser)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
