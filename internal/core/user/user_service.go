// Package user provides core business logic services for users.
package user

import (
	"errors"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

// Errors returned by Service.
var (
	// ErrUserNotFound is returned when the target user does not exist.
	ErrUserNotFound = errors.New("user not found")
	// ErrInvalidParam is returned when neither userId nor username is given.
	ErrInvalidParam = errors.New("userId or username is required")
)

// Service provides user-related business logic.
type Service struct {
	userRepo repository.UserRepository
}

// NewService creates a new user Service.
func NewService(userRepo repository.UserRepository) *Service {
	return &Service{userRepo: userRepo}
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
