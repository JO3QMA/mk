package repository

import (
	"context"
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cleanupMuting(t *testing.T, id string) {
	t.Helper()
	testDB.Exec(`DELETE FROM "muting" WHERE id = ?`, id)
}

func cleanupRenoteMuting(t *testing.T, id string) {
	t.Helper()
	testDB.Exec(`DELETE FROM "renote_muting" WHERE id = ?`, id)
}

func TestMutingRepository_CRUDExpiry(t *testing.T) {
	repo := NewMutingRepository(testDB)
	u1 := insertTestUser(t, "u_mt_1", "mt1")
	u2 := insertTestUser(t, "u_mt_2", "mt2")
	u3 := insertTestUser(t, "u_mt_3", "mt3")
	defer cleanupUser(t, u1.ID)
	defer cleanupUser(t, u2.ID)
	defer cleanupUser(t, u3.ID)

	// active mute (no expiry)
	active := &model.Muting{ID: "m_active", MuterID: u1.ID, MuteeID: u2.ID}
	require.NoError(t, repo.Create(active))
	defer cleanupMuting(t, active.ID)

	// expired mute
	past := time.Now().Add(-1 * time.Hour)
	expired := &model.Muting{ID: "m_expired", MuterID: u1.ID, MuteeID: u3.ID, ExpiresAt: &past}
	require.NoError(t, repo.Create(expired))
	defer cleanupMuting(t, expired.ID)

	exists, err := repo.Exists(u1.ID, u2.ID)
	require.NoError(t, err)
	assert.True(t, exists)

	// 期限切れはExistsで除外
	exists, err = repo.Exists(u1.ID, u3.ID)
	require.NoError(t, err)
	assert.False(t, exists)

	found, err := repo.FindByPair(u1.ID, u2.ID)
	require.NoError(t, err)
	assert.Equal(t, "m_active", found.ID)

	rows, err := repo.ListByMuter(u1.ID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, rows, 2)

	require.NoError(t, repo.Delete(active))
	_, err = repo.FindByPair(u1.ID, u2.ID)
	assert.Error(t, err)
}

func TestMutingRepository_QueryErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := NewMutingRepository(testDB.WithContext(ctx))

	err := repo.Create(&model.Muting{ID: "x", MuterID: "a", MuteeID: "b"})
	assert.Error(t, err)
	_, err = repo.FindByPair("a", "b")
	assert.Error(t, err)
	_, err = repo.Exists("a", "b")
	assert.Error(t, err)
	_, err = repo.ListByMuter("a", 10, 0)
	assert.Error(t, err)
}

func TestRenoteMutingRepository_CRUD(t *testing.T) {
	repo := NewRenoteMutingRepository(testDB)
	u1 := insertTestUser(t, "u_rmt_1", "rmt1")
	u2 := insertTestUser(t, "u_rmt_2", "rmt2")
	defer cleanupUser(t, u1.ID)
	defer cleanupUser(t, u2.ID)

	rec := &model.RenoteMuting{ID: "rm_1", MuterID: u1.ID, MuteeID: u2.ID}
	require.NoError(t, repo.Create(rec))
	defer cleanupRenoteMuting(t, rec.ID)

	found, err := repo.FindByPair(u1.ID, u2.ID)
	require.NoError(t, err)
	assert.Equal(t, "rm_1", found.ID)

	exists, err := repo.Exists(u1.ID, u2.ID)
	require.NoError(t, err)
	assert.True(t, exists)

	rows, err := repo.ListByMuter(u1.ID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, rows, 1)

	require.NoError(t, repo.Delete(rec))
	_, err = repo.FindByPair(u1.ID, u2.ID)
	assert.Error(t, err)
}

func TestRenoteMutingRepository_QueryErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := NewRenoteMutingRepository(testDB.WithContext(ctx))

	err := repo.Create(&model.RenoteMuting{ID: "x", MuterID: "a", MuteeID: "b"})
	assert.Error(t, err)
	_, err = repo.FindByPair("a", "b")
	assert.Error(t, err)
	_, err = repo.Exists("a", "b")
	assert.Error(t, err)
	_, err = repo.ListByMuter("a", 10, 0)
	assert.Error(t, err)
}
