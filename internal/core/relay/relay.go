// Package relay implements the ActivityPub relay subscription protocol.
// It ties together the local `relay` table, the instance's relay system
// actor (via core/systemaccount), and the outbound HTTP-signed delivery
// pipeline (via DeliverEnqueuer).
//
// Upstream reference: Misskey's core/RelayService.ts.
package relay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/shiroha-a/mk/internal/activitypub"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

// Relay statuses — mirror the upstream string values so admin UIs and
// Accept/Reject inbox handlers can interoperate.
const (
	StatusRequesting = "requesting"
	StatusAccepted   = "accepted"
	StatusRejected   = "rejected"
)

// SystemAccountFetcher abstracts core/systemaccount.Service so relay
// can be unit-tested without pulling in the user/keypair/SA repos.
type SystemAccountFetcher interface {
	Fetch(kind string) (*model.User, error)
}

// Deliverer is the subset of federation.DeliverService that relay
// depends on. Separated as an interface so mocks can record activity
// payloads without pulling the whole delivery pipeline.
type Deliverer interface {
	DeliverActivity(signerUserID string, body []byte, inboxes []string) error
}

// Service orchestrates add/remove/mark/list/deliver operations against
// the local relay table and the AP delivery pipeline.
type Service struct {
	repo     repository.RelayRepository
	sysAcct  SystemAccountFetcher
	renderer *activitypub.Renderer
	deliver  Deliverer
	idGen    id.Generator
	clock    func() time.Time

	// accepted list cache with 10-minute TTL, matching upstream.
	cacheMu     sync.RWMutex
	cacheLoaded time.Time
	cacheRelays []*model.Relay
	cacheMaxAge time.Duration
}

// NewService constructs a Service.
func NewService(
	repo repository.RelayRepository,
	sysAcct SystemAccountFetcher,
	renderer *activitypub.Renderer,
	deliver Deliverer,
	idGen id.Generator,
) *Service {
	return &Service{
		repo:        repo,
		sysAcct:     sysAcct,
		renderer:    renderer,
		deliver:     deliver,
		idGen:       idGen,
		clock:       time.Now,
		cacheMaxAge: 10 * time.Minute,
	}
}

// SetClock replaces the clock, primarily for tests.
func (s *Service) SetClock(now func() time.Time) {
	if now != nil {
		s.clock = now
	}
}

// SetCacheMaxAge overrides the default 10-minute TTL on the accepted
// relays cache. 0 以下は無視。
func (s *Service) SetCacheMaxAge(d time.Duration) {
	if d > 0 {
		s.cacheMaxAge = d
	}
}

// Add creates a relay row (status=requesting), renders a Follow
// activity signed by the relay actor, and enqueues it to the relay
// inbox. Returns the persisted row.
func (s *Service) Add(ctx context.Context, inbox string) (*model.Relay, error) {
	if inbox == "" {
		return nil, errors.New("relay: inbox is required")
	}
	rel := &model.Relay{
		ID:     s.idGen.Generate(s.clock()),
		Inbox:  inbox,
		Status: StatusRequesting,
	}
	if err := s.repo.Create(rel); err != nil {
		return nil, fmt.Errorf("relay: create row: %w", err)
	}
	if err := s.sendFollow(ctx, rel); err != nil {
		return nil, err
	}
	s.invalidateCache()
	return rel, nil
}

// Remove sends an Undo(Follow) to the relay inbox and deletes the row.
// 行が無ければ nil (idempotent)。
func (s *Service) Remove(ctx context.Context, id string) error {
	rel, err := s.repo.FindByID(id)
	if err != nil {
		return nil
	}
	if err := s.sendUndoFollow(ctx, rel); err != nil {
		return err
	}
	if err := s.repo.Delete(id); err != nil {
		return fmt.Errorf("relay: delete row: %w", err)
	}
	s.invalidateCache()
	return nil
}

// List returns every relay row (regardless of status).
func (s *Service) List(ctx context.Context) ([]*model.Relay, error) {
	return s.repo.List()
}

// MarkAccepted flips the status of the given relay to "accepted". Used
// by the inbox Accept handler when it detects a follow-relay activity.
func (s *Service) MarkAccepted(ctx context.Context, id string) error {
	if err := s.repo.UpdateStatus(id, StatusAccepted); err != nil {
		return err
	}
	s.invalidateCache()
	return nil
}

// MarkRejected flips the status to "rejected".
func (s *Service) MarkRejected(ctx context.Context, id string) error {
	if err := s.repo.UpdateStatus(id, StatusRejected); err != nil {
		return err
	}
	s.invalidateCache()
	return nil
}

// DeliverToAccepted enqueues activity to every accepted relay's inbox.
// Signer is the author of the activity (typically a local user posting
// a public note); the relay itself is not used as the signer because
// the relay actor would not be recognised as the activity's author.
//
// activity は JSON にシリアライズ可能な構造体 or map。
func (s *Service) DeliverToAccepted(ctx context.Context, signerUserID string, activity any) error {
	if activity == nil {
		return nil
	}
	relays, err := s.acceptedCached()
	if err != nil {
		return err
	}
	if len(relays) == 0 {
		return nil
	}
	body, err := json.Marshal(activity)
	if err != nil {
		return fmt.Errorf("relay: marshal activity: %w", err)
	}
	inboxes := make([]string, 0, len(relays))
	for _, r := range relays {
		inboxes = append(inboxes, r.Inbox)
	}
	return s.deliver.DeliverActivity(signerUserID, body, inboxes)
}

// sendFollow renders a Follow-Relay activity and enqueues it, signed by
// the relay system actor.
func (s *Service) sendFollow(ctx context.Context, rel *model.Relay) error {
	relayActor, err := s.sysAcct.Fetch("relay")
	if err != nil {
		return fmt.Errorf("relay: fetch relay actor: %w", err)
	}
	follow := s.renderer.RenderFollowRelay(rel.ID, relayActor.ID)
	body, err := json.Marshal(follow)
	if err != nil {
		return fmt.Errorf("relay: marshal follow: %w", err)
	}
	return s.deliver.DeliverActivity(relayActor.ID, body, []string{rel.Inbox})
}

// sendUndoFollow emits Undo(Follow) for the relay. The Follow is
// re-rendered from the stored relay id (the id format is stable) so
// callers do not need to have kept the original Follow object.
func (s *Service) sendUndoFollow(ctx context.Context, rel *model.Relay) error {
	relayActor, err := s.sysAcct.Fetch("relay")
	if err != nil {
		return fmt.Errorf("relay: fetch relay actor: %w", err)
	}
	follow := s.renderer.RenderFollowRelay(rel.ID, relayActor.ID)
	undo := map[string]any{
		"@context": follow.Context,
		"id":       follow.ID + "/undo",
		"type":     "Undo",
		"actor":    follow.Actor,
		"object":   follow,
	}
	body, err := json.Marshal(undo)
	if err != nil {
		return fmt.Errorf("relay: marshal undo: %w", err)
	}
	return s.deliver.DeliverActivity(relayActor.ID, body, []string{rel.Inbox})
}

// acceptedCached returns the cached list of accepted relays, refreshing
// from the DB when the cache is stale or empty.
func (s *Service) acceptedCached() ([]*model.Relay, error) {
	s.cacheMu.RLock()
	if s.cacheRelays != nil && s.clock().Sub(s.cacheLoaded) < s.cacheMaxAge {
		out := s.cacheRelays
		s.cacheMu.RUnlock()
		return out, nil
	}
	s.cacheMu.RUnlock()

	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	// 再チェック (他の goroutine が先に埋めたかも)
	if s.cacheRelays != nil && s.clock().Sub(s.cacheLoaded) < s.cacheMaxAge {
		return s.cacheRelays, nil
	}
	fresh, err := s.repo.ListByStatus(StatusAccepted)
	if err != nil {
		return nil, err
	}
	s.cacheRelays = fresh
	s.cacheLoaded = s.clock()
	return fresh, nil
}

func (s *Service) invalidateCache() {
	s.cacheMu.Lock()
	s.cacheRelays = nil
	s.cacheMu.Unlock()
}
