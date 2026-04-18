package repository

import (
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelMutingRepository_CRUD(t *testing.T) {
	repo := NewChannelMutingRepository(testDB)
	seedUser(t, "chmute_u1")
	seedChannel(t, "chmute_c1", "chmute_u1")
	t.Cleanup(func() { testDB.Exec(`DELETE FROM "channel_muting" WHERE "userId" = ?`, "chmute_u1") })

	m := &model.ChannelMuting{ID: "chmute_1", UserID: "chmute_u1", ChannelID: "chmute_c1"}
	require.NoError(t, repo.Create(m))

	ok, err := repo.Exists("chmute_u1", "chmute_c1")
	require.NoError(t, err)
	assert.True(t, ok)

	list, err := repo.ListByUser("chmute_u1")
	require.NoError(t, err)
	assert.Len(t, list, 1)

	require.NoError(t, repo.Delete("chmute_u1", "chmute_c1"))
	ok, err = repo.Exists("chmute_u1", "chmute_c1")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestChannelMutingRepository_ListEmpty(t *testing.T) {
	repo := NewChannelMutingRepository(testDB)
	list, err := repo.ListByUser("chmute_nonexistent")
	require.NoError(t, err)
	assert.Empty(t, list)
}
