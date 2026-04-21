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
	// frontend admin/job-queue.vue が期待する Misskey Bull shape
	// ({name, counts:{active,delayed,waiting,...}, metrics:..., db:...})
	// を返すことを検証する。
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
	assert.Equal(t, "deliver", deliver["name"])
	counts, ok := deliver["counts"].(map[string]any)
	require.True(t, ok)
	assert.EqualValues(t, 1, counts["active"])
	assert.EqualValues(t, 3, counts["waiting"])
	// scheduled(0) + retry(1) がdelayedに集約される。
	assert.EqualValues(t, 1, counts["delayed"])
}

func TestQueueQueueStats_SingleQueueQuery(t *testing.T) {
	// frontend の fetchCurrentQueue は {queue: "deliver"} を投げて
	// 単一 queue の shape を受ける。req.Queue が与えられた場合はその
	// queue 1 つ分を返す挙動を検証する。
	h, _, _, _ := newTestHandler(t)
	insp := &stubQueueInspector{
		queues: []string{"deliver", "push"},
		info: map[string]*apiadmin.QueueInfoResult{
			"deliver": {Queue: "deliver", Size: 5, Pending: 3, Active: 1},
			"push":    {Queue: "push"},
		},
	}
	h.SetQueueInspector(insp)
	rec := doPost(h.QueueQueueStats, `{"queue":"deliver"}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "deliver", got["name"])
	counts, ok := got["counts"].(map[string]any)
	require.True(t, ok)
	assert.EqualValues(t, 1, counts["active"])
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

// listDelayedTasks が scheduled と retry を結合する際に合計が limit を超え
// ないことを確認する (regression guard)。
func TestQueueDeliverDelayed_CappedAtLimit(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	scheduled := make([]*apiadmin.QueueTaskSummary, 0, 3)
	for _, id := range []string{"s1", "s2", "s3"} {
		scheduled = append(scheduled, &apiadmin.QueueTaskSummary{ID: id})
	}
	retry := make([]*apiadmin.QueueTaskSummary, 0, 3)
	for _, id := range []string{"r1", "r2", "r3"} {
		retry = append(retry, &apiadmin.QueueTaskSummary{ID: id})
	}
	insp := &stubQueueInspector{
		scheduled: map[string][]*apiadmin.QueueTaskSummary{"deliver": scheduled},
		retry:     map[string][]*apiadmin.QueueTaskSummary{"deliver": retry},
	}
	h.SetQueueInspector(insp)

	rec := doPost(h.QueueDeliverDelayed, `{"limit":4}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var rows []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rows))
	assert.Len(t, rows, 4, "combined output must not exceed requested limit")
	// scheduled を先に詰めるので s1..s3 が最初の 3 件、続いて r1 が 4 件目
	assert.Equal(t, "s1", rows[0]["id"])
	assert.Equal(t, "r1", rows[3]["id"])
}

// listDelayedTasks が scheduled/retry 境界を越えて正しくページングできること
// を確認する。旧実装では同じ req.Page を両リストに forward していたため、
// scheduled でページが埋まると retry 側の早い項目が永久に見えなくなる bug が
// あった (Devin #183 review)。
func TestQueueDeliverDelayed_PagingCrossesBoundary(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	scheduled := make([]*apiadmin.QueueTaskSummary, 0, 5)
	for _, id := range []string{"s1", "s2", "s3", "s4", "s5"} {
		scheduled = append(scheduled, &apiadmin.QueueTaskSummary{ID: id})
	}
	retry := make([]*apiadmin.QueueTaskSummary, 0, 3)
	for _, id := range []string{"r1", "r2", "r3"} {
		retry = append(retry, &apiadmin.QueueTaskSummary{ID: id})
	}
	insp := &stubQueueInspector{
		scheduled: map[string][]*apiadmin.QueueTaskSummary{"deliver": scheduled},
		retry:     map[string][]*apiadmin.QueueTaskSummary{"deliver": retry},
	}
	h.SetQueueInspector(insp)

	// page 1 limit 3 → s1, s2, s3
	rec := doPost(h.QueueDeliverDelayed, `{"limit":3,"page":1}`, adminUser)
	var rows []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rows))
	require.Len(t, rows, 3)
	assert.Equal(t, []any{"s1", "s2", "s3"}, []any{rows[0]["id"], rows[1]["id"], rows[2]["id"]})

	// page 2 limit 3 → s4, s5, r1 (境界をまたぐ)
	rec = doPost(h.QueueDeliverDelayed, `{"limit":3,"page":2}`, adminUser)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rows))
	require.Len(t, rows, 3)
	assert.Equal(t, []any{"s4", "s5", "r1"}, []any{rows[0]["id"], rows[1]["id"], rows[2]["id"]})

	// page 3 limit 3 → r2, r3
	rec = doPost(h.QueueDeliverDelayed, `{"limit":3,"page":3}`, adminUser)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rows))
	require.Len(t, rows, 2)
	assert.Equal(t, []any{"r2", "r3"}, []any{rows[0]["id"], rows[1]["id"]})

	// page 4 → 空
	rec = doPost(h.QueueDeliverDelayed, `{"limit":3,"page":4}`, adminUser)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rows))
	assert.Empty(t, rows)
}

// QueueJobs で queue 未指定時、複数キューを走査しても合計 limit を超えない
// ことを確認する。
func TestQueueJobs_MultiQueueCappedAtLimit(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	insp := &stubQueueInspector{
		queues: []string{"deliver", "inbox"},
		pending: map[string][]*apiadmin.QueueTaskSummary{
			"deliver": {{ID: "d1"}, {ID: "d2"}, {ID: "d3"}},
			"inbox":   {{ID: "i1"}, {ID: "i2"}},
		},
	}
	h.SetQueueInspector(insp)

	rec := doPost(h.QueueJobs, `{"limit":2}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var rows []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rows))
	assert.Len(t, rows, 2, "multi-queue fan-out must still respect limit")
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
