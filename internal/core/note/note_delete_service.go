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

// DeleteFederationHook is invoked after a note is deleted so that the
// ActivityPub layer can broadcast a Delete activity. パッケージ間の循環依存を
// 避けるためinterfaceで受け取る (実装は core/federation)。
type DeleteFederationHook interface {
	OnNoteDeleted(author *model.User, note *model.Note)
}

// DeleteService provides note deletion logic.
type DeleteService struct {
	noteRepo       repository.NoteRepository
	federationHook DeleteFederationHook
}

// NewDeleteService creates a new DeleteService.
func NewDeleteService(noteRepo repository.NoteRepository) *DeleteService {
	return &DeleteService{noteRepo: noteRepo}
}

// SetFederationHook attaches a DeleteFederationHook invoked after Delete.
func (s *DeleteService) SetFederationHook(h DeleteFederationHook) {
	s.federationHook = h
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

	if err := s.noteRepo.Delete(note); err != nil {
		return err
	}
	if s.federationHook != nil {
		s.federationHook.OnNoteDeleted(user, note)
	}
	return nil
}
