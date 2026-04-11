package repository

import (
	"testing"

	"github.com/lib/pq"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func TestReversiRepository_CRUD(t *testing.T) {
	repo := NewReversiRepository(testDB)
	u1 := insertTestUser(t, "u_rev_1", "reversiuser1")
	defer cleanupUser(t, u1.ID)
	u2 := insertTestUser(t, "u_rev_2", "reversiuser2")
	defer cleanupUser(t, u2.ID)

	game := &model.ReversiGame{
		ID:                   "rev_g1",
		User1ID:              u1.ID,
		User2ID:              u2.ID,
		Map:                  pq.StringArray{"--------", "--------", "--------", "---wb---", "---bw---", "--------", "--------", "--------"},
		BW:                   "random",
		TimeLimitForEachTurn: 90,
		Logs:                 datatypes.JSON("[]"),
	}
	require.NoError(t, repo.Create(game))
	defer testDB.Exec(`DELETE FROM "reversi_game" WHERE id = ?`, game.ID)

	// FindByID
	got, err := repo.FindByID(game.ID)
	require.NoError(t, err)
	assert.Equal(t, u1.ID, got.User1ID)

	// FindByID not found
	_, err = repo.FindByID("ghost")
	assert.Error(t, err)

	// Update + ListByUser
	got.BW = "1"
	require.NoError(t, repo.Update(got))
	list, err := repo.ListByUser(u1.ID, 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 1)

	// ListActive
	active, err := repo.ListActive()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(active), 1)

	// Delete
	require.NoError(t, repo.Delete(game.ID))
	_, err = repo.FindByID(game.ID)
	assert.Error(t, err)
}
