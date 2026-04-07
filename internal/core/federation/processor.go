package federation

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	corefollowing "github.com/shiroha-a/mk/internal/core/following"
	"github.com/shiroha-a/mk/internal/repository"
)

// Errors returned by Processor.
var (
	// ErrUnsupportedActivity is returned when an activity type cannot be handled.
	ErrUnsupportedActivity = errors.New("unsupported activity type")
)

// Processor dispatches inbound activities to the right handler.
type Processor struct {
	resolver         *Resolver
	followingService *corefollowing.Service
	userRepo         repository.UserRepository
}

// NewProcessor constructs a Processor.
func NewProcessor(
	resolver *Resolver,
	followingService *corefollowing.Service,
	userRepo repository.UserRepository,
) *Processor {
	return &Processor{
		resolver:         resolver,
		followingService: followingService,
		userRepo:         userRepo,
	}
}

// genericActivity is the minimum struct used by the dispatcher to read the
// activity type and actor.
type genericActivity struct {
	Type   string          `json:"type"`
	Actor  string          `json:"actor"`
	Object json.RawMessage `json:"object"`
	ID     string          `json:"id"`
}

// Process consumes a JSON activity body received in an inbox. Returns nil if
// the activity is accepted (whether or not it produced side effects). Returns
// ErrUnsupportedActivity for types that are accepted but currently not
// processed (callers should still acknowledge the request to the sender).
func (p *Processor) Process(body []byte) error {
	var act genericActivity
	if err := json.Unmarshal(body, &act); err != nil {
		return fmt.Errorf("invalid activity json: %w", err)
	}
	if act.Actor == "" {
		return errors.New("activity missing actor")
	}

	switch strings.ToLower(act.Type) {
	case "follow":
		return p.handleFollow(act)
	case "undo":
		return p.handleUndo(act)
	case "accept":
		return p.handleAccept(act)
	case "create", "like", "announce", "delete", "update", "reject":
		// 受け入れるが現状は no-op (Phase 3 の後続ステップで対応)
		return ErrUnsupportedActivity
	}
	return ErrUnsupportedActivity
}

// handleFollow processes an inbound Follow activity.
func (p *Processor) handleFollow(act genericActivity) error {
	follower, err := p.resolver.ResolveActor(act.Actor)
	if err != nil {
		return err
	}
	followeeURI, err := readObjectString(act.Object)
	if err != nil {
		return err
	}
	followee, err := p.userRepo.FindByURI(followeeURI)
	if err != nil {
		return errors.New("unknown followee")
	}
	if _, err := p.followingService.Follow(follower.ID, followee.ID); err != nil {
		// 既にフォロー済みは許容
		if errors.Is(err, corefollowing.ErrAlreadyFollowing) {
			return nil
		}
		return err
	}
	return nil
}

// handleUndo processes an Undo activity wrapping a Follow.
func (p *Processor) handleUndo(act genericActivity) error {
	var inner genericActivity
	if err := json.Unmarshal(act.Object, &inner); err != nil {
		return fmt.Errorf("invalid undo object: %w", err)
	}
	if !strings.EqualFold(inner.Type, "follow") {
		return ErrUnsupportedActivity
	}
	follower, err := p.resolver.ResolveActor(act.Actor)
	if err != nil {
		return err
	}
	followeeURI, err := readObjectString(inner.Object)
	if err != nil {
		return err
	}
	followee, err := p.userRepo.FindByURI(followeeURI)
	if err != nil {
		return errors.New("unknown followee")
	}
	if err := p.followingService.Unfollow(follower.ID, followee.ID); err != nil {
		if errors.Is(err, corefollowing.ErrNotFollowing) {
			return nil
		}
		return err
	}
	return nil
}

// handleAccept currently only logs that an Accept was received. リモートからの
// Acceptは主に follow request の承認なので、Step E 以降で完全実装する。
func (p *Processor) handleAccept(_ genericActivity) error {
	return nil
}

// readObjectString reads an activity Object field that is either a plain
// string IRI or a nested object with an "id" field.
func readObjectString(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", errors.New("missing object")
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil && s != "" {
		return s, nil
	}
	var obj struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", err
	}
	if obj.ID == "" {
		return "", errors.New("object missing id")
	}
	return obj.ID, nil
}

// publicKeyExtractor extracts the public key PEM from a fetched actor body.
// (used by the resolver to populate keys when adding remote users; future
// commits can move this to a richer Person → user mapper)
//
// 現状は inbox handler 側で activitypub.VerifyRequest を直接呼び、key fetch は
// resolver 経由で行う想定。
