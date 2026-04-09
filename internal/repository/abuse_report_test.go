package repository

import (
	"context"
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cleanupAbuseReport(t *testing.T, id string) {
	t.Helper()
	testDB.Exec(`DELETE FROM "abuse_user_report" WHERE id = ?`, id)
}

func cleanupModLog(t *testing.T, id string) {
	t.Helper()
	testDB.Exec(`DELETE FROM "moderation_log" WHERE id = ?`, id)
}

// --- AbuseReportRepository ---

func TestAbuseReportRepository_CRUD(t *testing.T) {
	repo := NewAbuseReportRepository(testDB)

	createTestUser(t, "ar_target")
	createTestUser(t, "ar_reporter")

	r := &model.AbuseUserReport{
		ID:           "ar_1",
		TargetUserID: "ar_target",
		ReporterID:   "ar_reporter",
		Comment:      "spam",
	}
	require.NoError(t, repo.Create(r))
	defer cleanupAbuseReport(t, r.ID)

	found, err := repo.FindByID(r.ID)
	require.NoError(t, err)
	assert.Equal(t, "spam", found.Comment)

	// List
	reports, err := repo.List(nil, 10, 0)
	require.NoError(t, err)
	assert.NotEmpty(t, reports)

	// List with resolved filter
	f := false
	reports, err = repo.List(&f, 10, 0)
	require.NoError(t, err)
	assert.NotEmpty(t, reports)

	// UpdateFields
	require.NoError(t, repo.UpdateFields(r.ID, map[string]any{"resolved": true}))
	found, _ = repo.FindByID(r.ID)
	assert.True(t, found.Resolved)
}

func TestAbuseReportRepository_List_Pagination(t *testing.T) {
	repo := NewAbuseReportRepository(testDB)
	createTestUser(t, "ar_p_t")
	createTestUser(t, "ar_p_r")

	r1 := &model.AbuseUserReport{ID: "ar_p1", TargetUserID: "ar_p_t", ReporterID: "ar_p_r"}
	r2 := &model.AbuseUserReport{ID: "ar_p2", TargetUserID: "ar_p_t", ReporterID: "ar_p_r"}
	require.NoError(t, repo.Create(r1))
	require.NoError(t, repo.Create(r2))
	defer cleanupAbuseReport(t, r1.ID)
	defer cleanupAbuseReport(t, r2.ID)

	reports, err := repo.List(nil, 1, 0)
	require.NoError(t, err)
	assert.Len(t, reports, 1)

	reports, err = repo.List(nil, 10, 1)
	require.NoError(t, err)
	assert.NotEmpty(t, reports)

	// limit cap
	reports, err = repo.List(nil, 999, 0)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(reports), 100)
}

func TestAbuseReportRepository_FindByID_NotFound(t *testing.T) {
	repo := NewAbuseReportRepository(testDB)
	_, err := repo.FindByID("ghost")
	assert.Error(t, err)
}

func TestAbuseReportRepository_List_ResolvedTrue(t *testing.T) {
	repo := NewAbuseReportRepository(testDB)
	tr := true
	reports, err := repo.List(&tr, 10, 0)
	require.NoError(t, err)
	_ = reports
}

func TestAbuseReportRepository_List_DefaultLimit(t *testing.T) {
	repo := NewAbuseReportRepository(testDB)
	reports, err := repo.List(nil, 0, 0) // limit=0 → default 10
	require.NoError(t, err)
	_ = reports
}

func TestModerationLogRepository_List_DefaultLimit(t *testing.T) {
	repo := NewModerationLogRepository(testDB)
	logs, err := repo.List(0, 0) // limit=0 → default 10
	require.NoError(t, err)
	_ = logs
}

func TestModerationLogRepository_List_LimitCap(t *testing.T) {
	repo := NewModerationLogRepository(testDB)
	logs, err := repo.List(999, 0) // > 100 → capped
	require.NoError(t, err)
	assert.LessOrEqual(t, len(logs), 100)
}

func TestAbuseReportRepository_List_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := NewAbuseReportRepository(testDB.WithContext(ctx))
	_, err := repo.List(nil, 10, 0)
	assert.Error(t, err)
}

// --- ModerationLogRepository ---

func TestModerationLogRepository_CRUD(t *testing.T) {
	repo := NewModerationLogRepository(testDB)

	createTestUser(t, "ml_user")

	log := &model.ModerationLog{
		ID:     "ml_1",
		UserID: "ml_user",
		Type:   "suspend",
		Info:   []byte(`{"targetUserId":"someone"}`),
	}
	require.NoError(t, repo.Create(log))
	defer cleanupModLog(t, log.ID)

	logs, err := repo.List(10, 0)
	require.NoError(t, err)
	assert.NotEmpty(t, logs)
}

func TestModerationLogRepository_List_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := NewModerationLogRepository(testDB.WithContext(ctx))
	_, err := repo.List(10, 0)
	assert.Error(t, err)
}

func TestModerationLogRepository_List_Pagination(t *testing.T) {
	repo := NewModerationLogRepository(testDB)
	createTestUser(t, "ml_pag")
	log1 := &model.ModerationLog{ID: "ml_p1", UserID: "ml_pag", Type: "a", Info: []byte("{}")}
	log2 := &model.ModerationLog{ID: "ml_p2", UserID: "ml_pag", Type: "b", Info: []byte("{}")}
	require.NoError(t, repo.Create(log1))
	require.NoError(t, repo.Create(log2))
	defer cleanupModLog(t, log1.ID)
	defer cleanupModLog(t, log2.ID)

	logs, err := repo.List(1, 0)
	require.NoError(t, err)
	assert.Len(t, logs, 1)

	logs, err = repo.List(10, 1)
	require.NoError(t, err)
	assert.NotEmpty(t, logs)
}
