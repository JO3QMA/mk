package repository

import (
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoteThreadMutingRepository_CRUD(t *testing.T) {
	repo := NewNoteThreadMutingRepository(testDB)
	seedUser(t, "ntm_u1")
	t.Cleanup(func() { testDB.Exec(`DELETE FROM "note_thread_muting" WHERE "userId" = ?`, "ntm_u1") })

	m := &model.NoteThreadMuting{ID: "ntm_1", UserID: "ntm_u1", ThreadID: "thread_1"}
	require.NoError(t, repo.Create(m))

	ok, err := repo.Exists("ntm_u1", "thread_1")
	require.NoError(t, err)
	assert.True(t, ok)

	require.NoError(t, repo.Delete("ntm_u1", "thread_1"))
	ok, err = repo.Exists("ntm_u1", "thread_1")
	require.NoError(t, err)
	assert.False(t, ok)
}
