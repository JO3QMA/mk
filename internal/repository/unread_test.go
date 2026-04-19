package repository

import (
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoteUnreadRepository_UpsertAndQueries(t *testing.T) {
	repo := NewNoteUnreadRepository(testDB)
	author := insertTestUser(t, "nu_author", "nuauthor")
	defer cleanupUser(t, author.ID)
	reader := insertTestUser(t, "nu_reader", "nureader")
	defer cleanupUser(t, reader.ID)

	note := &model.Note{ID: "nu_note_1", UserID: author.ID, Visibility: "specified"}
	require.NoError(t, testDB.Create(note).Error)
	defer testDB.Exec(`DELETE FROM "note" WHERE id = ?`, note.ID)
	defer testDB.Exec(`DELETE FROM "note_unread" WHERE "userId" = ?`, reader.ID)

	// 初回 insert: isSpecified=true / isMentioned=false
	require.NoError(t, repo.Upsert(&model.NoteUnread{
		ID: "nu_1", UserID: reader.ID, NoteID: note.ID, NoteUserID: author.ID,
		IsSpecified: true, IsMentioned: false,
	}))

	has, err := repo.HasAnySpecified(reader.ID)
	require.NoError(t, err)
	assert.True(t, has)
	hasM, err := repo.HasAnyMentioned(reader.ID)
	require.NoError(t, err)
	assert.False(t, hasM)

	// Upsert: 同一 (userId, noteId) で isMentioned=true → flag が OR で結合
	require.NoError(t, repo.Upsert(&model.NoteUnread{
		ID: "nu_dup", UserID: reader.ID, NoteID: note.ID, NoteUserID: author.ID,
		IsSpecified: false, IsMentioned: true,
	}))

	has, _ = repo.HasAnySpecified(reader.ID)
	assert.True(t, has, "isSpecified は OR 結合で true のまま")
	hasM, _ = repo.HasAnyMentioned(reader.ID)
	assert.True(t, hasM, "isMentioned は true に昇格")

	// delete 後は has=false
	require.NoError(t, repo.DeleteByUserNote(reader.ID, note.ID))
	has, _ = repo.HasAnySpecified(reader.ID)
	assert.False(t, has)
	hasM, _ = repo.HasAnyMentioned(reader.ID)
	assert.False(t, hasM)
}

func TestNoteUnreadRepository_HasAny_Empty(t *testing.T) {
	repo := NewNoteUnreadRepository(testDB)
	has, err := repo.HasAnySpecified("no-such-user")
	require.NoError(t, err)
	assert.False(t, has)
	hasM, err := repo.HasAnyMentioned("no-such-user")
	require.NoError(t, err)
	assert.False(t, hasM)
}
