package note_test

import (
	"errors"
	"testing"

	"github.com/shiroha-a/mk/internal/core/note"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteService_Success(t *testing.T) {
	noteRepo := testutil.NewMockNoteRepository()
	noteRepo.Notes["note1"] = &model.Note{ID: "note1", UserID: "user1"}
	svc := note.NewDeleteService(noteRepo)

	err := svc.Delete(&model.User{ID: "user1"}, "note1")
	require.NoError(t, err)
	assert.Empty(t, noteRepo.Notes)
}

func TestDeleteService_NotFound(t *testing.T) {
	noteRepo := testutil.NewMockNoteRepository()
	svc := note.NewDeleteService(noteRepo)

	err := svc.Delete(&model.User{ID: "user1"}, "missing")
	require.Error(t, err)
	assert.True(t, errors.Is(err, note.ErrNoteNotFound))
}

func TestDeleteService_AccessDenied(t *testing.T) {
	noteRepo := testutil.NewMockNoteRepository()
	noteRepo.Notes["note1"] = &model.Note{ID: "note1", UserID: "owner"}
	svc := note.NewDeleteService(noteRepo)

	err := svc.Delete(&model.User{ID: "intruder"}, "note1")
	require.Error(t, err)
	assert.True(t, errors.Is(err, note.ErrNoteAccessDenied))
	assert.Len(t, noteRepo.Notes, 1)
}

func TestDeleteService_NilUser(t *testing.T) {
	noteRepo := testutil.NewMockNoteRepository()
	svc := note.NewDeleteService(noteRepo)

	err := svc.Delete(nil, "x")
	require.Error(t, err)
}

// failingDeleteRepo wraps MockNoteRepository to fail on Delete only.
type failingDeleteRepo struct {
	*testutil.MockNoteRepository
}

func (r *failingDeleteRepo) Delete(_ *model.Note) error {
	return errors.New("delete failed")
}

func TestDeleteService_RepoDeleteError(t *testing.T) {
	mockRepo := testutil.NewMockNoteRepository()
	mockRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "user1"}
	svc := note.NewDeleteService(&failingDeleteRepo{MockNoteRepository: mockRepo})
	err := svc.Delete(&model.User{ID: "user1"}, "n1")
	require.Error(t, err)
}

// recordingDeleteFederationHook captures delete federation calls.
type recordingDeleteFederationHook struct {
	called bool
	author *model.User
	note   *model.Note
}

func (h *recordingDeleteFederationHook) OnNoteDeleted(author *model.User, n *model.Note) {
	h.called = true
	h.author = author
	h.note = n
}

func TestDeleteService_FederationHookInvoked(t *testing.T) {
	noteRepo := testutil.NewMockNoteRepository()
	noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "user1"}
	svc := note.NewDeleteService(noteRepo)
	hook := &recordingDeleteFederationHook{}
	svc.SetFederationHook(hook)

	require.NoError(t, svc.Delete(&model.User{ID: "user1"}, "n1"))
	assert.True(t, hook.called)
	assert.Equal(t, "n1", hook.note.ID)
}
