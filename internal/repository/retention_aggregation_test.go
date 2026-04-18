package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetentionAggregationRepository_ListRecent(t *testing.T) {
	repo := NewRetentionAggregationRepository(testDB)
	// DEFAULT値に依存してINSERT (createdAt / updatedAt / data / userIds / usersCount)
	require.NoError(t, testDB.Exec(
		`INSERT INTO "retention_aggregation" (id, "dateKey") VALUES (?, ?) ON CONFLICT (id) DO NOTHING`,
		"ra_1", "2026-04-18",
	).Error)
	t.Cleanup(func() { testDB.Exec(`DELETE FROM "retention_aggregation" WHERE id = ?`, "ra_1") })

	records, err := repo.ListRecent(10)
	require.NoError(t, err)
	assert.NotEmpty(t, records)
}
