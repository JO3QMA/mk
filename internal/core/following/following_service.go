// Package following provides the UserFollowingService for managing follow
// relationships and follow requests.
package following

import (
	"errors"
	"time"

	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

// Errors returned by Service.
var (
	// ErrAlreadyFollowing is returned when the follower already follows the followee.
	ErrAlreadyFollowing = errors.New("already following")
	// ErrNotFollowing is returned when there is no following relationship to delete.
	ErrNotFollowing = errors.New("not following")
	// ErrSelfFollow is returned when a user attempts to follow themselves.
	ErrSelfFollow = errors.New("cannot follow yourself")
	// ErrFolloweeNotFound is returned when the target user does not exist.
	ErrFolloweeNotFound = errors.New("followee not found")
	// ErrRequestNotFound is returned when the follow request does not exist.
	ErrRequestNotFound = errors.New("follow request not found")
	// ErrAlreadyRequested is returned when a follow request already exists.
	ErrAlreadyRequested = errors.New("already requested")
)

// FollowResult represents the outcome of a Follow call.
type FollowResult struct {
	// Following is non-nil when the relationship was created directly.
	Following *model.Following
	// Request is non-nil when a follow request was created instead of following directly.
	Request *model.FollowRequest
}

// Service manages following relationships and follow requests.
type Service struct {
	userRepo          repository.UserRepository
	followingRepo     repository.FollowingRepository
	followRequestRepo repository.FollowRequestRepository
	idGen             id.Generator
}

// NewService creates a new following Service.
func NewService(
	userRepo repository.UserRepository,
	followingRepo repository.FollowingRepository,
	followRequestRepo repository.FollowRequestRepository,
	idGen id.Generator,
) *Service {
	return &Service{
		userRepo:          userRepo,
		followingRepo:     followingRepo,
		followRequestRepo: followRequestRepo,
		idGen:             idGen,
	}
}

// Follow creates a following relationship from follower to followee.
// If the followee is locked, a FollowRequest is created instead and the
// FollowResult contains Request set instead of Following.
//
// Phase 2 Step B implementation only handles local follows. Federation
// (HTTP signatures, AP delivery), block checks, and notifications are
// added in later phases.
func (s *Service) Follow(followerID, followeeID string) (*FollowResult, error) {
	if followerID == followeeID {
		return nil, ErrSelfFollow
	}

	followee, err := s.userRepo.FindByID(followeeID)
	if err != nil {
		return nil, ErrFolloweeNotFound
	}
	follower, err := s.userRepo.FindByID(followerID)
	if err != nil {
		return nil, ErrFolloweeNotFound
	}

	// 既存のフォロー関係をチェック
	if exists, err := s.followingRepo.Exists(followerID, followeeID); err != nil {
		return nil, err
	} else if exists {
		return nil, ErrAlreadyFollowing
	}

	// Lockedアカウントへのフォローはリクエスト扱い
	if followee.IsLocked {
		// 既存リクエストがあればエラー
		if exists, err := s.followRequestRepo.Exists(followerID, followeeID); err != nil {
			return nil, err
		} else if exists {
			return nil, ErrAlreadyRequested
		}
		req := &model.FollowRequest{
			ID:           s.idGen.Generate(time.Now()),
			FollowerID:   followerID,
			FolloweeID:   followeeID,
			FollowerHost: follower.Host,
			FolloweeHost: followee.Host,
		}
		if err := s.followRequestRepo.Create(req); err != nil {
			return nil, err
		}
		return &FollowResult{Request: req}, nil
	}

	f := &model.Following{
		ID:           s.idGen.Generate(time.Now()),
		FollowerID:   followerID,
		FolloweeID:   followeeID,
		FollowerHost: follower.Host,
		FolloweeHost: followee.Host,
	}
	if err := s.followingRepo.Create(f); err != nil {
		return nil, err
	}

	// counterの更新は失敗しても致命ではないが、現状はerror伝播
	if err := s.userRepo.IncrementFollowingCount(followerID, 1); err != nil {
		return nil, err
	}
	if err := s.userRepo.IncrementFollowersCount(followeeID, 1); err != nil {
		return nil, err
	}

	return &FollowResult{Following: f}, nil
}

// Unfollow removes a following relationship from follower to followee.
func (s *Service) Unfollow(followerID, followeeID string) error {
	if followerID == followeeID {
		return ErrSelfFollow
	}

	f, err := s.followingRepo.FindByPair(followerID, followeeID)
	if err != nil {
		return ErrNotFollowing
	}
	if err := s.followingRepo.Delete(f); err != nil {
		return err
	}
	if err := s.userRepo.IncrementFollowingCount(followerID, -1); err != nil {
		return err
	}
	if err := s.userRepo.IncrementFollowersCount(followeeID, -1); err != nil {
		return err
	}
	return nil
}

// AcceptRequest accepts a pending follow request, deleting the request and
// creating a Following relationship.
func (s *Service) AcceptRequest(followeeID, followerID string) error {
	req, err := s.followRequestRepo.FindByPair(followerID, followeeID)
	if err != nil {
		return ErrRequestNotFound
	}
	if err := s.followRequestRepo.Delete(req); err != nil {
		return err
	}
	f := &model.Following{
		ID:           s.idGen.Generate(time.Now()),
		FollowerID:   req.FollowerID,
		FolloweeID:   req.FolloweeID,
		FollowerHost: req.FollowerHost,
		FolloweeHost: req.FolloweeHost,
	}
	if err := s.followingRepo.Create(f); err != nil {
		return err
	}
	if err := s.userRepo.IncrementFollowingCount(req.FollowerID, 1); err != nil {
		return err
	}
	if err := s.userRepo.IncrementFollowersCount(req.FolloweeID, 1); err != nil {
		return err
	}
	return nil
}

// RejectRequest rejects a pending follow request received by the followee.
func (s *Service) RejectRequest(followeeID, followerID string) error {
	req, err := s.followRequestRepo.FindByPair(followerID, followeeID)
	if err != nil {
		return ErrRequestNotFound
	}
	return s.followRequestRepo.Delete(req)
}

// CancelRequest cancels an outgoing follow request created by the follower.
func (s *Service) CancelRequest(followerID, followeeID string) error {
	req, err := s.followRequestRepo.FindByPair(followerID, followeeID)
	if err != nil {
		return ErrRequestNotFound
	}
	return s.followRequestRepo.Delete(req)
}

// ListReceivedRequests returns follow requests received by userID.
func (s *Service) ListReceivedRequests(userID string, limit, offset int) ([]*model.FollowRequest, error) {
	return s.followRequestRepo.ListReceived(userID, limit, offset)
}

// ListSentRequests returns follow requests sent by userID.
func (s *Service) ListSentRequests(userID string, limit, offset int) ([]*model.FollowRequest, error) {
	return s.followRequestRepo.ListSent(userID, limit, offset)
}
