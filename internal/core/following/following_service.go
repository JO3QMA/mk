// Package following provides the UserFollowingService for managing follow
// relationships and follow requests.
package following

import (
	"errors"
	"time"

	"github.com/shiroha-a/mk/internal/entity"
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

// ChartHook is invoked after follow / unfollow events so the chart
// subsystem can record per-user and per-instance follow deltas.
// パッケージ間の循環依存を避けるため interface で受け取る (実装は
// core/chart/charthook)。
type ChartHook interface {
	OnFollow(follower, followee *model.User)
	OnUnfollow(follower, followee *model.User)
}

// WebhookHook is invoked after a follow / unfollow / follow-accepted event so
// that user webhooks subscribed to `follow` / `unfollow` / `followed` can fire.
// 循環依存を避けるため interface で受け取る (実装は core/webhook)。
type WebhookHook interface {
	OnFollow(follower, followee *model.User)
	OnUnfollow(follower, followee *model.User)
	OnFollowed(follower, followee *model.User)
}

// MainStreamPublisher emits real-time events to a single target user's `main`
// WebSocket channel. Used here to deliver `follow`, `followed`, `unfollow`,
// and `receiveFollowRequest` events so the frontend can reflect follow state
// changes immediately. 循環依存を避けるため interface で受け取る (実装は
// internal/stream)。
type MainStreamPublisher interface {
	PublishMainEvent(userID, eventType string, body any)
}

// Service manages following relationships and follow requests.
type Service struct {
	userRepo            repository.UserRepository
	followingRepo       repository.FollowingRepository
	followRequestRepo   repository.FollowRequestRepository
	idGen               id.Generator
	notificationHook    NotificationHook
	blockingChecker     BlockingChecker
	federationHook      FederationHook
	chartHook           ChartHook
	webhookHook         WebhookHook
	mainStreamPublisher MainStreamPublisher
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

// SetChartHook attaches a ChartHook invoked after follow / unfollow.
func (s *Service) SetChartHook(h ChartHook) {
	s.chartHook = h
}

// SetWebhookHook attaches a WebhookHook invoked after follow / unfollow /
// follow-accepted events so user webhooks subscribed to follow events fire.
func (s *Service) SetWebhookHook(h WebhookHook) {
	s.webhookHook = h
}

// SetMainStreamPublisher attaches a publisher used to emit `main` channel
// events (`follow`, `followed`, `unfollow`, `receiveFollowRequest`). Optional —
// nil disables emit.
func (s *Service) SetMainStreamPublisher(p MainStreamPublisher) {
	s.mainStreamPublisher = p
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
		// TS本家: UserFollowingService は `receiveFollowRequest` を
		// followee の main stream に publish する。body は follower User。
		// UserLite で送るのは他のWSイベント (notification内のuser等) と
		// 揃える意図。Instance情報は PackUserLite に含まれないが、
		// frontend は undefined 許容。
		if s.mainStreamPublisher != nil {
			s.mainStreamPublisher.PublishMainEvent(followeeID, "receiveFollowRequest", entity.PackUserLite(follower))
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
	if s.chartHook != nil {
		s.chartHook.OnFollow(follower, followee)
	}
	// Webhookはfollower側で `follow`、followee側で `followed` を発火する。
	if s.webhookHook != nil {
		s.webhookHook.OnFollow(follower, followee)
		s.webhookHook.OnFollowed(follower, followee)
	}
	// TS本家 UserFollowingService.follow() は follower の main に `follow`
	// (相手側の User)、followee の main に `followed` (自分を follow した
	// User) を publish する。frontend MkFollowButton.onFollowChangeが
	// body.isFollowing / body.hasPendingFollowRequestFromYouを読むため、
	// UserDetailed shapeでpackしてviewer依存フィールドも埋めておく。
	if s.mainStreamPublisher != nil {
		s.mainStreamPublisher.PublishMainEvent(followerID, "follow", entity.PackUserForFollowStreamEvent(followee, true, false))
		s.mainStreamPublisher.PublishMainEvent(followeeID, "followed", entity.PackUserLite(follower))
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
	// hook 呼び出しに必要なユーザー情報を一度だけロードして使い回す。
	// 失敗してもベストエフォートで continue する。
	if s.federationHook != nil || s.chartHook != nil || s.webhookHook != nil || s.mainStreamPublisher != nil {
		follower, ferr := s.userRepo.FindByID(followerID)
		followee, eerr := s.userRepo.FindByID(followeeID)
		if ferr == nil && eerr == nil {
			if s.federationHook != nil {
				s.federationHook.OnLocalUnfollowed(follower, followee)
			}
			if s.chartHook != nil {
				s.chartHook.OnUnfollow(follower, followee)
			}
			if s.webhookHook != nil {
				s.webhookHook.OnUnfollow(follower, followee)
			}
			// TS本家は自分が unfollow した相手を main に publish する
			// (フォローボタン等の即時反映)。follow event と同様、UserDetailed
			// shapeでisFollowing=false / hasPendingFollowRequestFromYou=falseを
			// 明示的に埋める (frontendはこれらを直接代入するのでundefined不可)。
			if s.mainStreamPublisher != nil {
				s.mainStreamPublisher.PublishMainEvent(followerID, "unfollow", entity.PackUserForFollowStreamEvent(followee, false, false))
			}
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
	if s.federationHook != nil || s.chartHook != nil || s.webhookHook != nil || s.mainStreamPublisher != nil {
		follower, ferr := s.userRepo.FindByID(req.FollowerID)
		followee, eerr := s.userRepo.FindByID(req.FolloweeID)
		if ferr == nil && eerr == nil {
			if s.federationHook != nil {
				s.federationHook.OnLocalFollowAccepted(follower, followee)
			}
			if s.chartHook != nil {
				s.chartHook.OnFollow(follower, followee)
			}
			if s.webhookHook != nil {
				s.webhookHook.OnFollow(follower, followee)
				s.webhookHook.OnFollowed(follower, followee)
			}
			// Accept によって Following が成立するので、Follow() と同じく
			// follower の main に `follow`、followee の main に `followed`
			// を publish する (TS本家 UserFollowingService.acceptFollow
			// と同等)。follow event body は UserDetailed + isFollowing=true。
			if s.mainStreamPublisher != nil {
				s.mainStreamPublisher.PublishMainEvent(req.FollowerID, "follow", entity.PackUserForFollowStreamEvent(followee, true, false))
				s.mainStreamPublisher.PublishMainEvent(req.FolloweeID, "followed", entity.PackUserLite(follower))
			}
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

// UpdateRelation applies partial updates to a single follower/followee pair.
// Used by following/update to toggle per-relation flags (notify, withReplies).
// Returns nil if the relation does not exist (no-op).
func (s *Service) UpdateRelation(followerID, followeeID string, fields map[string]any) error {
	return s.followingRepo.UpdateRelation(followerID, followeeID, fields)
}

// UpdateAllByFollower applies partial updates to every Following row whose
// followerId matches. Used by following/update-all.
func (s *Service) UpdateAllByFollower(followerID string, fields map[string]any) error {
	return s.followingRepo.UpdateAllByFollower(followerID, fields)
}
