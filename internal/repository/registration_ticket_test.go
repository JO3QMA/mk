package repository

import (
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cleanupInvite(t *testing.T, ids ...string) {
	t.Helper()
	for _, id := range ids {
		testDB.Exec(`DELETE FROM "registration_ticket" WHERE id = ?`, id)
	}
}

func TestRegistrationTicketRepository_CreateAndListAll(t *testing.T) {
	repo := NewRegistrationTicketRepository(testDB)

	// 前回の失敗テスト残骸を掃除してから始める。
	cleanupInvite(t, "rt_unused", "rt_used", "rt_exp")

	// usedBy は FK 制約 → user テーブルに実体が必要。テスト用の user を作る。
	u := insertTestUser(t, "rt_user", "rtu")
	defer cleanupUser(t, u.ID)

	unused := &model.RegistrationTicket{ID: "rt_unused", Code: "uuu-unused"}
	usedAt := time.Now().UTC().Truncate(time.Second)
	used := &model.RegistrationTicket{ID: "rt_used", Code: "uuu-used", UsedByID: &u.ID, UsedAt: &usedAt}
	past := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
	expired := &model.RegistrationTicket{ID: "rt_exp", Code: "uuu-exp", ExpiresAt: &past}

	require.NoError(t, repo.Create(unused))
	require.NoError(t, repo.Create(used))
	require.NoError(t, repo.Create(expired))
	defer cleanupInvite(t, unused.ID, used.ID, expired.ID)

	all, err := repo.List(RegistrationTicketAll, 100, 0, time.Now())
	require.NoError(t, err)
	ids := collectIDs(all)
	assert.Contains(t, ids, "rt_unused")
	assert.Contains(t, ids, "rt_used")
	assert.Contains(t, ids, "rt_exp")

	unusedRows, err := repo.List(RegistrationTicketUnused, 100, 0, time.Now())
	require.NoError(t, err)
	assert.Contains(t, collectIDs(unusedRows), "rt_unused")
	assert.NotContains(t, collectIDs(unusedRows), "rt_used")

	usedRows, err := repo.List(RegistrationTicketUsed, 100, 0, time.Now())
	require.NoError(t, err)
	assert.Contains(t, collectIDs(usedRows), "rt_used")
	assert.NotContains(t, collectIDs(usedRows), "rt_unused")

	expiredRows, err := repo.List(RegistrationTicketExpired, 100, 0, time.Now())
	require.NoError(t, err)
	assert.Contains(t, collectIDs(expiredRows), "rt_exp")
	assert.NotContains(t, collectIDs(expiredRows), "rt_unused")
}

func TestRegistrationTicketRepository_LimitOffsetDefaults(t *testing.T) {
	repo := NewRegistrationTicketRepository(testDB)
	// 負数 / 0 は default に丸められる
	_, err := repo.List(RegistrationTicketAll, 0, -1, time.Now())
	require.NoError(t, err)
}

func collectIDs(rows []*model.RegistrationTicket) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.ID
	}
	return out
}
