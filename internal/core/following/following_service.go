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
	// ErrBlocked is returned when either party blocks the other.
	ErrBlocked = errors.New("blocking relationship prevents this operation")
)

// BlockingChecker reports whether one user has blocked another. パッケージ間の
// 循環依存を避けるためinterfaceで受け取る (実装は core/blocking)。
type BlockingChecker interface {
	IsBlocked(blockerID, blockeeID string) (bool, error)
}

// FollowResult represents the outcome of a Follow call.
type FollowResult struct {
	// Following is non-nil when the relationship was created directly.
	Following *model.Following
	// Request is non-nil when a follow request was created instead of following directly.
	Request *model.FollowRequest
}

// NotificationHook is invoked after follow/follow-request events to create
// notification entries. パッケージ間の循環依存を避けるためinterfaceで受け取る。
type NotificationHook interface {
	OnFollowed(followerID, followeeID string)
	OnFollowRequested(followerID, followeeID string)
	OnFollowAccepted(followerID, followeeID string)
}

// FederationHook is invoked after follow/unfollow/accept events involving a
// remote user so that the ActivityPub layer can deliver Follow/Undo/Accept
// activities. パッケージ間の循環依存を避けるためinterfaceで受け取る。
type FederationHook interface {
	OnLocalFollowed(follower, followee *model.User)
	OnLocalUnfollowed(follower, followee *model.User)
	OnLocalFollowAccepted(follower, followee *model.User)
}

// Service manages following relationships and follow requests.
type Service struct {
	userRepo          repository.UserRepository
	followingRepo     repository.FollowingRepository
	followRequestRepo repository.FollowRequestRepository
	idGen             id.Generator
	notificationHook  NotificationHook
	blockingChecker   BlockingChecker
	federationHook    FederationHook
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

// SetNotificationHook attaches a NotificationHook used by Follow/AcceptRequest.
func (s *Service) SetNotificationHook(h NotificationHook) {
	s.notificationHook = h
}

// SetBlockingChecker attaches a BlockingChecker used by Follow.
func (s *Service) SetBlockingChecker(c BlockingChecker) {
	s.blockingChecker = c
}

// SetFederationHook attaches a FederationHook used to dispatch Follow / Undo /
// Accept activities to remote inboxes.
func (s *Service) SetFederationHook(h FederationHook) {
	s.federationHook = h
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

	// ブロック関係があるとフォロー不可 (双方向で確認)
	if s.blockingChecker != nil {
		if blocked, err := s.blockingChecker.IsBlocked(followeeID, followerID); err != nil {
			return nil, err
		} else if blocked {
			return nil, ErrBlocked
		}
		if blocked, err := s.blockingChecker.IsBlocked(followerID, followeeID); err != nil {
			return nil, err
		} else if blocked {
			return nil, ErrBlocked
		}
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
		if s.notificationHook != nil {
			s.notificationHook.OnFollowRequested(followerID, followeeID)
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

	if s.notificationHook != nil {
		s.notificationHook.OnFollowed(followerID, followeeID)
	}
	if s.federationHook != nil {
		s.federationHook.OnLocalFollowed(follower, followee)
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
	if s.federationHook != nil {
		// hook 呼び出しに必要なユーザー情報を取得する。失敗してもベストエフォートで
		// continue する。
		follower, ferr := s.userRepo.FindByID(followerID)
		followee, eerr := s.userRepo.FindByID(followeeID)
		if ferr == nil && eerr == nil {
			s.federationHook.OnLocalUnfollowed(follower, followee)
		}
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
	if s.notificationHook != nil {
		s.notificationHook.OnFollowAccepted(req.FollowerID, req.FolloweeID)
	}
	if s.federationHook != nil {
		follower, ferr := s.userRepo.FindByID(req.FollowerID)
		followee, eerr := s.userRepo.FindByID(req.FolloweeID)
		if ferr == nil && eerr == nil {
			s.federationHook.OnLocalFollowAccepted(follower, followee)
		}
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

// ListReceivedFollowing returns followings where userID is the followee.
// 「userIDをフォローしているユーザー」(=followers) を返す。
func (s *Service) ListReceivedFollowing(userID string, limit, offset int) ([]*model.Following, error) {
	return s.followingRepo.ListFollowers(userID, limit, offset)
}

// ListSentFollowing returns followings where userID is the follower.
// 「userIDがフォローしているユーザー」を返す。
func (s *Service) ListSentFollowing(userID string, limit, offset int) ([]*model.Following, error) {
	return s.followingRepo.ListFollowing(userID, limit, offset)
}
