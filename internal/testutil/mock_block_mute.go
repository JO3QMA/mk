package testutil

import (
	"time"

	"github.com/shiroha-a/mk/internal/model"
)

// MockBlockingRepository is a test double for repository.BlockingRepository.
type MockBlockingRepository struct {
	Blockings map[string]*model.Blocking // keyed by ID
}

func NewMockBlockingRepository() *MockBlockingRepository {
	return &MockBlockingRepository{Blockings: make(map[string]*model.Blocking)}
}

func (m *MockBlockingRepository) Create(b *model.Blocking) error {
	m.Blockings[b.ID] = b
	return nil
}

func (m *MockBlockingRepository) Delete(b *model.Blocking) error {
	delete(m.Blockings, b.ID)
	return nil
}

func (m *MockBlockingRepository) FindByPair(blockerID, blockeeID string) (*model.Blocking, error) {
	for _, b := range m.Blockings {
		if b.BlockerID == blockerID && b.BlockeeID == blockeeID {
			return b, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MockBlockingRepository) Exists(blockerID, blockeeID string) (bool, error) {
	_, err := m.FindByPair(blockerID, blockeeID)
	return err == nil, nil
}

func (m *MockBlockingRepository) ListByBlocker(blockerID string, limit, offset int) ([]*model.Blocking, error) {
	var rows []*model.Blocking
	for _, b := range m.Blockings {
		if b.BlockerID == blockerID {
			rows = append(rows, b)
		}
	}
	return paginateBlockings(rows, limit, offset), nil
}

// MockMutingRepository is a test double for repository.MutingRepository.
type MockMutingRepository struct {
	Mutings map[string]*model.Muting
}

func NewMockMutingRepository() *MockMutingRepository {
	return &MockMutingRepository{Mutings: make(map[string]*model.Muting)}
}

func (m *MockMutingRepository) Create(rec *model.Muting) error {
	m.Mutings[rec.ID] = rec
	return nil
}

func (m *MockMutingRepository) Delete(rec *model.Muting) error {
	delete(m.Mutings, rec.ID)
	return nil
}

func (m *MockMutingRepository) FindByPair(muterID, muteeID string) (*model.Muting, error) {
	for _, r := range m.Mutings {
		if r.MuterID == muterID && r.MuteeID == muteeID {
			return r, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MockMutingRepository) Exists(muterID, muteeID string) (bool, error) {
	now := time.Now()
	for _, r := range m.Mutings {
		if r.MuterID == muterID && r.MuteeID == muteeID {
			if r.ExpiresAt != nil && r.ExpiresAt.Before(now) {
				return false, nil
			}
			return true, nil
		}
	}
	return false, nil
}

func (m *MockMutingRepository) ListByMuter(muterID string, limit, offset int) ([]*model.Muting, error) {
	var rows []*model.Muting
	for _, r := range m.Mutings {
		if r.MuterID == muterID {
			rows = append(rows, r)
		}
	}
	return paginateMutings(rows, limit, offset), nil
}

// MockRenoteMutingRepository is a test double for repository.RenoteMutingRepository.
type MockRenoteMutingRepository struct {
	Mutings map[string]*model.RenoteMuting
}

func NewMockRenoteMutingRepository() *MockRenoteMutingRepository {
	return &MockRenoteMutingRepository{Mutings: make(map[string]*model.RenoteMuting)}
}

func (m *MockRenoteMutingRepository) Create(rec *model.RenoteMuting) error {
	m.Mutings[rec.ID] = rec
	return nil
}

func (m *MockRenoteMutingRepository) Delete(rec *model.RenoteMuting) error {
	delete(m.Mutings, rec.ID)
	return nil
}

func (m *MockRenoteMutingRepository) FindByPair(muterID, muteeID string) (*model.RenoteMuting, error) {
	for _, r := range m.Mutings {
		if r.MuterID == muterID && r.MuteeID == muteeID {
			return r, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MockRenoteMutingRepository) Exists(muterID, muteeID string) (bool, error) {
	_, err := m.FindByPair(muterID, muteeID)
	return err == nil, nil
}

func (m *MockRenoteMutingRepository) ListByMuter(muterID string, limit, offset int) ([]*model.RenoteMuting, error) {
	var rows []*model.RenoteMuting
	for _, r := range m.Mutings {
		if r.MuterID == muterID {
			rows = append(rows, r)
		}
	}
	return paginateRenoteMutings(rows, limit, offset), nil
}

func paginateBlockings(rows []*model.Blocking, limit, offset int) []*model.Blocking {
	if offset >= len(rows) {
		return nil
	}
	end := min(offset+limit, len(rows))
	return rows[offset:end]
}

func paginateMutings(rows []*model.Muting, limit, offset int) []*model.Muting {
	if offset >= len(rows) {
		return nil
	}
	end := min(offset+limit, len(rows))
	return rows[offset:end]
}

func paginateRenoteMutings(rows []*model.RenoteMuting, limit, offset int) []*model.RenoteMuting {
	if offset >= len(rows) {
		return nil
	}
	end := min(offset+limit, len(rows))
	return rows[offset:end]
}
