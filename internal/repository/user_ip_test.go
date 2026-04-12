package repository

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserIPRepository_Upsert(t *testing.T) {
	repo := NewUserIPRepository(testDB)
	u := insertTestUser(t, "ip_u1", "ipu1")
	defer cleanupUser(t, u.ID)

	require.NoError(t, repo.Upsert(u.ID, "192.168.1.1"))
	require.NoError(t, repo.Upsert(u.ID, "10.0.0.1"))

	ips, err := repo.ListByUser(u.ID, 10)
	require.NoError(t, err)
	assert.Len(t, ips, 2)

	// upsert same IP updates timestamp
	time.Sleep(10 * time.Millisecond)
	require.NoError(t, repo.Upsert(u.ID, "192.168.1.1"))
	ips, _ = repo.ListByUser(u.ID, 10)
	assert.Len(t, ips, 2)

	// cleanup
	testDB.Exec(`DELETE FROM "user_ip" WHERE "userId" = ?`, u.ID)
}

func TestUserIPRepository_ListByUser_Empty(t *testing.T) {
	repo := NewUserIPRepository(testDB)
	ips, err := repo.ListByUser("ghost-ip-user", 10)
	require.NoError(t, err)
	assert.Empty(t, ips)
}

func TestUserIPRepository_ListByUser_DefaultLimit(t *testing.T) {
	repo := NewUserIPRepository(testDB)
	ips, err := repo.ListByUser("ghost", 0)
	require.NoError(t, err)
	assert.Empty(t, ips)
}
