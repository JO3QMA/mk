// Package blocking provides UserBlockingService for managing user blocks.
package blocking

import (
	"errors"
	"time"

	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

// Errors returned by Service.
var (
	// ErrSelfBlock is returned when a user attempts to block themselves.
	ErrSelfBlock = errors.New("cannot block yourself")
	// ErrBlockeeNotFound is returned when the target user does not exist.
	ErrBlockeeNotFound = errors.New("blockee not found")
	// ErrAlreadyBlocking is returned when the blocker already blocks the blockee.
	ErrAlreadyBlocking = errors.New("already blocking")
	// ErrNotBlocking is returned when there is no blocking relationship to delete.
	ErrNotBlocking = errors.New("not blocking")
)

// Service manages user blocking relationships.
type Service struct {
	userRepo      repository.UserRepository
	blockingRepo  repository.BlockingRepository
	followingRepo repository.FollowingRepository
	instanceRepo  repository.InstanceRepository // optional, for #596 incremental counters
	idGen         id.Generator
}

// NewService constructs a UserBlockingService.
// followingRepo は省略可だが、指定されているとブロック時に既存のフォロー関係を
// 自動解除する。
func NewService(
	userRepo repository.UserRepository,
	blockingRepo repository.BlockingRepository,
	followingRepo repository.FollowingRepository,
	idGen id.Generator,
) *Service {
	return &Service{
		userRepo:      userRepo,
		blockingRepo:  blockingRepo,
		followingRepo: followingRepo,
		idGen:         idGen,
	}
}

// SetInstanceRepo wires an InstanceRepository so the auto-unfollow side effect
// of Block also adjusts remote instance followers/following counts (#596)。
// 未配線でも本機能には影響しない (起動時 RecomputeFollowCounts が安全網)。
func (s *Service) SetInstanceRepo(r repository.InstanceRepository) {
	s.instanceRepo = r
}

// Block creates a blocking relationship from blocker to blockee.
// 既存のフォロー関係 (双方向) があれば自動的に解除する。
func (s *Service) Block(blockerID, blockeeID string) (*model.Blocking, error) {
	if blockerID == blockeeID {
		return nil, ErrSelfBlock
	}
	if _, err := s.userRepo.FindByID(blockeeID); err != nil {
		return nil, ErrBlockeeNotFound
	}

	if exists, err := s.blockingRepo.Exists(blockerID, blockeeID); err != nil {
		return nil, err
	} else if exists {
		return nil, ErrAlreadyBlocking
	}

	b := &model.Blocking{
		ID:        s.idGen.Generate(time.Now()),
		BlockerID: blockerID,
		BlockeeID: blockeeID,
	}
	if err := s.blockingRepo.Create(b); err != nil {
		return nil, err
	}

	// 既存のフォロー関係を双方向で解除する。fold-in counterも調整する。
	if s.followingRepo != nil {
		s.removeFollowing(blockerID, blockeeID)
		s.removeFollowing(blockeeID, blockerID)
	}

	return b, nil
}

// Unblock removes a blocking relationship.
func (s *Service) Unblock(blockerID, blockeeID string) error {
	if blockerID == blockeeID {
		return ErrSelfBlock
	}
	b, err := s.blockingRepo.FindByPair(blockerID, blockeeID)
	if err != nil {
		return ErrNotBlocking
	}
	return s.blockingRepo.Delete(b)
}

// IsBlocked reports whether blocker has blocked blockee.
func (s *Service) IsBlocked(blockerID, blockeeID string) (bool, error) {
	return s.blockingRepo.Exists(blockerID, blockeeID)
}

// List returns the user's blockings with cursor (sinceID/untilID) or offset
// pagination. Cursor 指定時は offset 無視 (upstream makePaginationQuery と一致)。
func (s *Service) List(blockerID, sinceID, untilID string, limit, offset int) ([]*model.Blocking, error) {
	if limit <= 0 {
		limit = 10
	}
	return s.blockingRepo.ListByBlocker(blockerID, sinceID, untilID, limit, offset)
}

// removeFollowing deletes a follow edge if it exists and adjusts counters.
// エラーは握り潰し (ベストエフォート)。
func (s *Service) removeFollowing(followerID, followeeID string) {
	f, err := s.followingRepo.FindByPair(followerID, followeeID)
	if err != nil {
		return
	}
	if err := s.followingRepo.Delete(f); err != nil {
		return
	}
	_ = s.userRepo.IncrementFollowingCount(followerID, -1)
	_ = s.userRepo.IncrementFollowersCount(followeeID, -1)
	// remote instance の集計列も -1 する (#596)。block→自動 unfollow 経路で
	// follow row が消えるので following.Service.Unfollow と同じ調整が必要。
	if s.instanceRepo != nil {
		if f.FollowerHost != nil {
			_ = s.instanceRepo.IncrementFollowersCount(*f.FollowerHost, -1)
		}
		if f.FolloweeHost != nil {
			_ = s.instanceRepo.IncrementFollowingCount(*f.FolloweeHost, -1)
		}
	}
}
