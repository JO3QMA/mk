package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsedUsernameRepository_CreateAndExists(t *testing.T) {
	repo := NewUsedUsernameRepository(testDB)
	t.Cleanup(func() { testDB.Exec(`DELETE FROM "used_username" WHERE username = ?`, "taken_name") })

	exists, err := repo.Exists("taken_name")
	require.NoError(t, err)
	assert.False(t, exists, "should not exist before Create")

	require.NoError(t, repo.Create("taken_name"))

	exists, err = repo.Exists("taken_name")
	require.NoError(t, err)
	assert.True(t, exists)
}
