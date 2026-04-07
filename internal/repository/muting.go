package repository

import (
	"time"

	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// MutingRepository provides data access for the `muting` table.
type MutingRepository interface {
	Create(m *model.Muting) error
	Delete(m *model.Muting) error
	FindByPair(muterID, muteeID string) (*model.Muting, error)
	Exists(muterID, muteeID string) (bool, error)
	ListByMuter(muterID string, limit, offset int) ([]*model.Muting, error)
}

type mutingRepository struct {
	db *gorm.DB
}

// NewMutingRepository creates a new MutingRepository.
func NewMutingRepository(db *gorm.DB) MutingRepository {
	return &mutingRepository{db: db}
}

func (r *mutingRepository) Create(m *model.Muting) error {
	return r.db.Create(m).Error
}

func (r *mutingRepository) Delete(m *model.Muting) error {
	return r.db.Delete(m).Error
}

func (r *mutingRepository) FindByPair(muterID, muteeID string) (*model.Muting, error) {
	var m model.Muting
	if err := r.db.Where("\"muterId\" = ? AND \"muteeId\" = ?", muterID, muteeID).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// Exists reports whether muter has an active (non-expired) mute on mutee.
func (r *mutingRepository) Exists(muterID, muteeID string) (bool, error) {
	var count int64
	now := time.Now()
	if err := r.db.Model(&model.Muting{}).
		Where("\"muterId\" = ? AND \"muteeId\" = ?", muterID, muteeID).
		Where("\"expiresAt\" IS NULL OR \"expiresAt\" > ?", now).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *mutingRepository) ListByMuter(muterID string, limit, offset int) ([]*model.Muting, error) {
	var rows []*model.Muting
	if err := r.db.Where("\"muterId\" = ?", muterID).
		Order("id DESC").
		Limit(limit).
		Offset(offset).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// RenoteMutingRepository provides data access for the `renote_muting` table.
type RenoteMutingRepository interface {
	Create(m *model.RenoteMuting) error
	Delete(m *model.RenoteMuting) error
	FindByPair(muterID, muteeID string) (*model.RenoteMuting, error)
	Exists(muterID, muteeID string) (bool, error)
	ListByMuter(muterID string, limit, offset int) ([]*model.RenoteMuting, error)
}

type renoteMutingRepository struct {
	db *gorm.DB
}

// NewRenoteMutingRepository creates a new RenoteMutingRepository.
func NewRenoteMutingRepository(db *gorm.DB) RenoteMutingRepository {
	return &renoteMutingRepository{db: db}
}

func (r *renoteMutingRepository) Create(m *model.RenoteMuting) error {
	return r.db.Create(m).Error
}

func (r *renoteMutingRepository) Delete(m *model.RenoteMuting) error {
	return r.db.Delete(m).Error
}

func (r *renoteMutingRepository) FindByPair(muterID, muteeID string) (*model.RenoteMuting, error) {
	var m model.RenoteMuting
	if err := r.db.Where("\"muterId\" = ? AND \"muteeId\" = ?", muterID, muteeID).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *renoteMutingRepository) Exists(muterID, muteeID string) (bool, error) {
	var count int64
	if err := r.db.Model(&model.RenoteMuting{}).
		Where("\"muterId\" = ? AND \"muteeId\" = ?", muterID, muteeID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *renoteMutingRepository) ListByMuter(muterID string, limit, offset int) ([]*model.RenoteMuting, error) {
	var rows []*model.RenoteMuting
	if err := r.db.Where("\"muterId\" = ?", muterID).
		Order("id DESC").
		Limit(limit).
		Offset(offset).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
