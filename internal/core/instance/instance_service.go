// Package instance provides services for managing remote ActivityPub
// instances and their lifecycle metadata.
package instance

import (
	"errors"
	"slices"
	"time"

	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

// Errors returned by Service.
var (
	// ErrInstanceNotFound is returned when no instance row matches the host.
	ErrInstanceNotFound = errors.New("instance not found")
)

// Service manages the local view of remote instances. It is the entry point
// for any code that needs to know whether a host is blocked / silenced /
// suspended, and for refreshing the cached metadata after a fetch.
type Service struct {
	repo     repository.InstanceRepository
	metaRepo repository.MetaRepository
	idGen    id.Generator
	clock    func() time.Time
}

// NewService constructs an instance Service.
func NewService(repo repository.InstanceRepository, metaRepo repository.MetaRepository, idGen id.Generator) *Service {
	return &Service{
		repo:     repo,
		metaRepo: metaRepo,
		idGen:    idGen,
		clock:    time.Now,
	}
}

// SetClock overrides the time source. Intended for tests.
func (s *Service) SetClock(now func() time.Time) {
	if now != nil {
		s.clock = now
	}
}

// RegisterFromHost ensures an instance row exists for the given host. If a row
// already exists it is returned as-is; otherwise a fresh row is created with
// firstRetrievedAt set to now and usersCount = 1.
//
// 呼び出し元: Resolver でリモートユーザーを新規取り込みした直後。
func (s *Service) RegisterFromHost(host string) (*model.Instance, error) {
	if host == "" {
		return nil, errors.New("host is required")
	}
	if existing, err := s.repo.FindByHost(host); err == nil {
		return existing, nil
	}
	now := s.clock()
	inst := &model.Instance{
		ID:               s.idGen.Generate(now),
		Host:             host,
		FirstRetrievedAt: now,
		UsersCount:       1,
		SuspensionState:  model.SuspensionStateNone,
	}
	if err := s.repo.Create(inst); err != nil {
		return nil, err
	}
	return inst, nil
}

// MarkRequestReceived bumps the latestRequestReceivedAt timestamp on the
// instance row. ホストが未登録の場合は黙って no-op。Inbox handler から呼ぶ。
func (s *Service) MarkRequestReceived(host string) error {
	if host == "" {
		return nil
	}
	if _, err := s.repo.FindByHost(host); err != nil {
		return nil
	}
	now := s.clock()
	return s.repo.UpdateFields(host, map[string]any{
		"latestRequestReceivedAt": &now,
	})
}

// RecordResponseSuccess marks the instance as responsive again, clearing
// notRespondingSince. Outbox の配信成功時 (Phase 4 で wire 予定) に呼ばれる。
func (s *Service) RecordResponseSuccess(host string) error {
	if host == "" {
		return nil
	}
	if _, err := s.repo.FindByHost(host); err != nil {
		return nil
	}
	return s.repo.UpdateFields(host, map[string]any{
		"isNotResponding":    false,
		"notRespondingSince": (*time.Time)(nil),
	})
}

// RecordResponseError marks the instance as not responding. 既に not responding
// 状態であれば notRespondingSince を更新せず維持する。
func (s *Service) RecordResponseError(host string) error {
	if host == "" {
		return nil
	}
	inst, err := s.repo.FindByHost(host)
	if err != nil {
		return nil
	}
	if inst.IsNotResponding {
		return nil
	}
	now := s.clock()
	return s.repo.UpdateFields(host, map[string]any{
		"isNotResponding":    true,
		"notRespondingSince": &now,
	})
}

// IsBlocked reports whether the host appears in meta.blockedHosts.
// meta が読めない場合は false を返す (ベストエフォート)。
func (s *Service) IsBlocked(host string) bool {
	if host == "" {
		return false
	}
	meta, err := s.metaRepo.Fetch()
	if err != nil {
		return false
	}
	return slices.Contains(meta.BlockedHosts, host)
}

// IsSilenced reports whether the host appears in meta.silencedHosts.
func (s *Service) IsSilenced(host string) bool {
	if host == "" {
		return false
	}
	meta, err := s.metaRepo.Fetch()
	if err != nil {
		return false
	}
	return slices.Contains(meta.SilencedHosts, host)
}

// Suspend updates the suspensionState column for the host. 引数の state には
// model.SuspensionStateManuallySuspended などを渡す。
func (s *Service) Suspend(host string, state model.SuspensionState) error {
	if host == "" {
		return errors.New("host is required")
	}
	if _, err := s.repo.FindByHost(host); err != nil {
		return ErrInstanceNotFound
	}
	return s.repo.UpdateFields(host, map[string]any{
		"suspensionState": state,
	})
}

// UpdateModerationNote sets the moderationNote field on the instance row.
func (s *Service) UpdateModerationNote(host, note string) error {
	if host == "" {
		return errors.New("host is required")
	}
	if _, err := s.repo.FindByHost(host); err != nil {
		return ErrInstanceNotFound
	}
	return s.repo.UpdateFields(host, map[string]any{
		"moderationNote": note,
	})
}

// FindByHost returns the instance row for the given host or ErrInstanceNotFound.
func (s *Service) FindByHost(host string) (*model.Instance, error) {
	inst, err := s.repo.FindByHost(host)
	if err != nil {
		return nil, ErrInstanceNotFound
	}
	return inst, nil
}

// List returns instances matching the filter.
func (s *Service) List(filter model.InstanceListFilter) ([]*model.Instance, error) {
	return s.repo.List(filter)
}
