// Package user provides core business logic services for users.
package user

import (
	"errors"
	"strings"
	"time"

	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

// MaxPinnedNotes is the upper limit on pinned notes per user.
// Misskey本家のデフォルトと同じく5件。
const MaxPinnedNotes = 5

// Errors returned by Service.
var (
	// ErrUserNotFound is returned when the target user does not exist.
	ErrUserNotFound = errors.New("user not found")
	// ErrInvalidParam is returned when neither userId nor username is given.
	ErrInvalidParam = errors.New("userId or username is required")
	// ErrNoteNotFound is returned when the target note does not exist or is
	// not owned by the requesting user.
	ErrNoteNotFound = errors.New("note not found")
	// ErrAlreadyPinned is returned when pinning an already-pinned note.
	ErrAlreadyPinned = errors.New("already pinned")
	// ErrPinLimitExceeded is returned when the user already has MaxPinnedNotes notes pinned.
	ErrPinLimitExceeded = errors.New("pin limit exceeded")
	// ErrPinNotFound is returned when unpinning a note that is not pinned.
	ErrPinNotFound = errors.New("pin not found")
)

// Service provides user-related business logic.
type Service struct {
	userRepo   repository.UserRepository
	noteRepo   repository.NoteRepository
	piningRepo repository.UserNotePiningRepository
	idGen      id.Generator
}

// NewService creates a new user Service.
// noteRepo, piningRepo, idGen are optional for callers that only need the
// read-only show methods (pass nil); the pin-related methods require them.
func NewService(
	userRepo repository.UserRepository,
	noteRepo repository.NoteRepository,
	piningRepo repository.UserNotePiningRepository,
	idGen id.Generator,
) *Service {
	return &Service{
		userRepo:   userRepo,
		noteRepo:   noteRepo,
		piningRepo: piningRepo,
		idGen:      idGen,
	}
}

// UserWithProfile bundles a user and its profile for handlers.
type UserWithProfile struct {
	User    *model.User
	Profile *model.UserProfile
}

// ShowByID returns the user (and profile) for the given ID.
func (s *Service) ShowByID(id string) (*UserWithProfile, error) {
	u, err := s.userRepo.FindByID(id)
	if err != nil {
		return nil, ErrUserNotFound
	}
	// Profileの取得失敗は致命ではないので無視する
	profile, _ := s.userRepo.FindProfileByUserID(u.ID)
	return &UserWithProfile{User: u, Profile: profile}, nil
}

// ShowByUsername returns the user (and profile) for the given username and host.
func (s *Service) ShowByUsername(username string, host *string) (*UserWithProfile, error) {
	u, err := s.userRepo.FindByUsernameLower(username, host)
	if err != nil {
		return nil, ErrUserNotFound
	}
	profile, _ := s.userRepo.FindProfileByUserID(u.ID)
	return &UserWithProfile{User: u, Profile: profile}, nil
}

// GetProfile returns the profile for the given user ID, or nil if not found.
func (s *Service) GetProfile(userID string) *model.UserProfile {
	profile, err := s.userRepo.FindProfileByUserID(userID)
	if err != nil {
		return nil
	}
	return profile
}

// Search returns users whose username matches the prefix query.
// 空のクエリは空のリストを返す。
func (s *Service) Search(query string, limit, offset int) ([]*model.User, error) {
	q := strings.TrimSpace(strings.TrimPrefix(query, "@"))
	if q == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}
	return s.userRepo.SearchByUsername(strings.ToLower(q), limit, offset)
}

// UpdateInput represents the editable fields of a user profile.
// Each pointer is interpreted as "leave unchanged when nil".
type UpdateInput struct {
	Name              **string
	Description       **string
	Location          **string
	Birthday          **string
	Lang              **string
	IsLocked          *bool
	IsBot             *bool
	IsCat             *bool
	IsExplorable      *bool
	HideOnlineStatus  *bool
	AlwaysMarkNsfw    *bool
	AutoSensitive     *bool
	NoCrawle          *bool
	PreventAiLearning *bool
}

// UpdateProfile applies the non-nil fields to the user and user_profile rows.
func (s *Service) UpdateProfile(userID string, in UpdateInput) (*UserWithProfile, error) {
	if _, err := s.userRepo.FindByID(userID); err != nil {
		return nil, ErrUserNotFound
	}

	userFields := map[string]any{}
	profileFields := map[string]any{}

	if in.Name != nil {
		userFields["name"] = *in.Name
	}
	if in.IsLocked != nil {
		userFields["isLocked"] = *in.IsLocked
	}
	if in.IsBot != nil {
		userFields["isBot"] = *in.IsBot
	}
	if in.IsCat != nil {
		userFields["isCat"] = *in.IsCat
	}
	if in.IsExplorable != nil {
		userFields["isExplorable"] = *in.IsExplorable
	}
	if in.HideOnlineStatus != nil {
		userFields["hideOnlineStatus"] = *in.HideOnlineStatus
	}
	if in.Description != nil {
		profileFields["description"] = *in.Description
	}
	if in.Location != nil {
		profileFields["location"] = *in.Location
	}
	if in.Birthday != nil {
		profileFields["birthday"] = *in.Birthday
	}
	if in.Lang != nil {
		profileFields["lang"] = *in.Lang
	}
	if in.AlwaysMarkNsfw != nil {
		profileFields["alwaysMarkNsfw"] = *in.AlwaysMarkNsfw
	}
	if in.AutoSensitive != nil {
		profileFields["autoSensitive"] = *in.AutoSensitive
	}
	if in.NoCrawle != nil {
		profileFields["noCrawle"] = *in.NoCrawle
	}
	if in.PreventAiLearning != nil {
		profileFields["preventAiLearning"] = *in.PreventAiLearning
	}

	if err := s.userRepo.UpdateUser(userID, userFields); err != nil {
		return nil, err
	}
	if err := s.userRepo.UpdateProfile(userID, profileFields); err != nil {
		return nil, err
	}

	return s.ShowByID(userID)
}

// PinNote pins the given note to the user's profile.
// Returns ErrNoteNotFound if the note doesn't exist or isn't owned by the user,
// ErrAlreadyPinned, or ErrPinLimitExceeded.
func (s *Service) PinNote(userID, noteID string) error {
	note, err := s.noteRepo.FindByID(noteID)
	if err != nil {
		return ErrNoteNotFound
	}
	if note.UserID != userID {
		return ErrNoteNotFound
	}

	if _, err := s.piningRepo.FindByPair(userID, noteID); err == nil {
		return ErrAlreadyPinned
	}

	count, err := s.piningRepo.CountByUser(userID)
	if err != nil {
		return err
	}
	if count >= MaxPinnedNotes {
		return ErrPinLimitExceeded
	}

	p := &model.UserNotePining{
		ID:     s.idGen.Generate(time.Now()),
		UserID: userID,
		NoteID: noteID,
	}
	return s.piningRepo.Create(p)
}

// UnpinNote removes a pinning entry. Returns ErrPinNotFound if the user has
// not pinned the given note.
func (s *Service) UnpinNote(userID, noteID string) error {
	p, err := s.piningRepo.FindByPair(userID, noteID)
	if err != nil {
		return ErrPinNotFound
	}
	return s.piningRepo.Delete(p)
}

// ListPinnedNotes returns the notes pinned by userID, in pinning order
// (most recent pin first).
func (s *Service) ListPinnedNotes(userID string) ([]*model.Note, error) {
	pinings, err := s.piningRepo.ListByUser(userID)
	if err != nil {
		return nil, err
	}
	if len(pinings) == 0 {
		return nil, nil
	}
	ids := make([]string, 0, len(pinings))
	for _, p := range pinings {
		ids = append(ids, p.NoteID)
	}
	return s.noteRepo.FindManyByIDsWithUser(ids)
}
