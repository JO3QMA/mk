// Package move implements i/move (account migration) business logic:
// destination URI resolution via the federation Resolver, alsoKnownAs
// verification, local user row updates (movedToUri/movedAt/alsoKnownAs),
// and Move activity delivery to follower inboxes.
package move

import (
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/shiroha-a/mk/internal/activitypub"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

// Errors returned by Service.Move.
var (
	// ErrNoSuchUser is returned when the destination URI cannot be resolved.
	ErrNoSuchUser = errors.New("no such user")
	// ErrAlreadyMoved is returned when the source user already has movedToUri set.
	ErrAlreadyMoved = errors.New("already moved")
	// ErrDestinationForbids is returned when the destination does not declare
	// the source URI in its alsoKnownAs, or the destination itself has already
	// moved elsewhere.
	ErrDestinationForbids = errors.New("destination account forbids")
	// ErrURINull is returned when the source or destination user lacks a URI.
	ErrURINull = errors.New("user URI is null")
	// ErrRemoteSourceForbidden is returned when the caller is not a local user.
	// AP 配送のため秘密鍵が必要で、リモートユーザーを move 元にはできない。
	ErrRemoteSourceForbidden = errors.New("source must be a local user")
)

// Resolver fetches a remote actor by URI and returns the persisted local
// representation. Implemented by federation.Resolver; kept as a local
// interface so Service doesn't import federation (avoids circular deps during
// wiring).
type Resolver interface {
	ResolveActor(uri string) (*model.User, error)
}

// Deliverer delivers a pre-rendered activity body to the follower inboxes of
// signerUserID. Implemented by federation.DeliverService.
type Deliverer interface {
	DeliverToFollowers(signerUserID string, body []byte) error
}

// Service owns the account-move workflow.
type Service struct {
	userRepo      repository.UserRepository
	followingRepo repository.FollowingRepository
	urls          *activitypub.URLBuilder
	renderer      *activitypub.Renderer
	resolver      Resolver
	deliverer     Deliverer
}

// NewService constructs a Service. resolver / deliverer may be nil in tests;
// Move returns ErrNoSuchUser early if resolver is missing, and skips delivery
// if deliverer is missing.
func NewService(
	userRepo repository.UserRepository,
	followingRepo repository.FollowingRepository,
	urls *activitypub.URLBuilder,
	renderer *activitypub.Renderer,
	resolver Resolver,
	deliverer Deliverer,
) *Service {
	return &Service{
		userRepo:      userRepo,
		followingRepo: followingRepo,
		urls:          urls,
		renderer:      renderer,
		resolver:      resolver,
		deliverer:     deliverer,
	}
}

// Move migrates src to the user identified by dstURI.
//
// Preconditions (mirror upstream AccountMoveService.moveFromLocal):
//   - src is a local user with a URI
//   - src.movedToUri is empty
//   - resolver.ResolveActor(dstURI) returns a user whose alsoKnownAs contains
//     src's canonical URI and whose movedToUri is empty
//
// Side effects on success:
//   - user row updated: movedToUri, movedAt, alsoKnownAs (ensures dstURI is
//     in the csv list so remote instances treat the migration as bidirectional)
//   - Move activity rendered and enqueued to every remote follower inbox
func (s *Service) Move(src *model.User, dstURI string) error {
	if src == nil {
		return ErrNoSuchUser
	}
	if !src.IsLocal() {
		return ErrRemoteSourceForbidden
	}
	if src.MovedToURI != nil && *src.MovedToURI != "" {
		return ErrAlreadyMoved
	}
	dstURI = strings.TrimSpace(dstURI)
	if dstURI == "" {
		return ErrNoSuchUser
	}
	srcURI := s.urls.UserURI(src.ID)

	if s.resolver == nil {
		return ErrNoSuchUser
	}
	dst, err := s.resolver.ResolveActor(dstURI)
	if err != nil || dst == nil {
		return ErrNoSuchUser
	}
	// 解決後の dst URI は resolver が正規化した形 (actor.id) を使う。
	// これが空なら遷移先が AP 的に不正。
	dstCanonical := ""
	if dst.URI != nil {
		dstCanonical = *dst.URI
	}
	if dstCanonical == "" {
		dstCanonical = dstURI
	}
	if !alsoKnownAsIncludes(dst.AlsoKnownAs, srcURI) {
		return ErrDestinationForbids
	}
	if dst.MovedToURI != nil && *dst.MovedToURI != "" {
		return ErrDestinationForbids
	}

	now := time.Now()
	newAlsoKnownAs := appendIfMissing(src.AlsoKnownAs, dstCanonical)
	fields := map[string]any{
		"movedToUri":  dstCanonical,
		"movedAt":     now,
		"alsoKnownAs": newAlsoKnownAs,
	}
	if err := s.userRepo.UpdateUser(src.ID, fields); err != nil {
		return err
	}
	// 呼び出し側が返り値で最新状態を参照できるよう、local struct も in-place 更新する。
	src.MovedToURI = &dstCanonical
	src.MovedAt = &now
	src.AlsoKnownAs = strPtr(newAlsoKnownAs)

	if s.deliverer == nil {
		return nil
	}
	move := s.renderer.RenderMove(src, dstCanonical)
	body, err := json.Marshal(move)
	if err != nil {
		return err
	}
	// DB は既にコミット済みなので、配送失敗で handler がエラーを返して再試行
	// されると次回 ErrAlreadyMoved で詰む。follower 通知は best-effort にし、
	// 失敗は log に落として握り潰す (note_delivery_hook と同じパターン)。
	if err := s.deliverer.DeliverToFollowers(src.ID, body); err != nil {
		slog.Warn("move: deliver to followers failed (DB already committed)",
			"srcID", src.ID, "dstURI", dstCanonical, "err", err)
	}
	return nil
}

// alsoKnownAsIncludes returns true if the csv alsoKnownAs field contains uri.
// 本家の alsoKnownAs は配列だが Go 側は text csv として格納している点に注意。
// Move() の呼び出し側が uri != "" を保証しているため、ここでは csv 側だけ
// 空チェックする。
func alsoKnownAsIncludes(csvPtr *string, uri string) bool {
	if csvPtr == nil || *csvPtr == "" {
		return false
	}
	for _, part := range strings.Split(*csvPtr, ",") {
		if strings.TrimSpace(part) == uri {
			return true
		}
	}
	return false
}

// appendIfMissing returns the csv with uri appended if not already present.
// 既存要素は順序を保ったまま残す (remote の expectation を壊さないため)。
// 呼び出し元 Move() が uri != "" を保証しているので空文字のハンドリングはしない。
func appendIfMissing(csvPtr *string, uri string) string {
	if csvPtr == nil || *csvPtr == "" {
		return uri
	}
	for _, part := range strings.Split(*csvPtr, ",") {
		if strings.TrimSpace(part) == uri {
			return *csvPtr
		}
	}
	return *csvPtr + "," + uri
}

func strPtr(s string) *string { return &s }
