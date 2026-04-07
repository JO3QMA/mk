package note

import (
	"errors"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

// Errors returned by DeleteService.
var (
	// ErrNoteNotFound is returned when the target note does not exist.
	ErrNoteNotFound = errors.New("note not found")
	// ErrNoteAccessDenied is returned when the user is not allowed to delete the note.
	ErrNoteAccessDenied = errors.New("not the author of this note")
)

// DeleteService provides note deletion logic.
type DeleteService struct {
	noteRepo repository.NoteRepository
}

// NewDeleteService creates a new DeleteService.
func NewDeleteService(noteRepo repository.NoteRepository) *DeleteService {
	return &DeleteService{noteRepo: noteRepo}
}

// Delete removes a note authored by the given user. It returns
// ErrNoteNotFound when the note does not exist and ErrNoteAccessDenied when
// the user is not the author.
func (s *DeleteService) Delete(user *model.User, noteID string) error {
	if user == nil {
		return errors.New("user is required")
	}

	note, err := s.noteRepo.FindByID(noteID)
	if err != nil {
		return ErrNoteNotFound
	}

	if note.UserID != user.ID {
		return ErrNoteAccessDenied
	}

	return s.noteRepo.Delete(note)
}
