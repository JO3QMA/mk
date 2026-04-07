package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// BlockingRepository provides data access for the `blocking` table.
type BlockingRepository interface {
	Create(b *model.Blocking) error
	Delete(b *model.Blocking) error
	FindByPair(blockerID, blockeeID string) (*model.Blocking, error)
	Exists(blockerID, blockeeID string) (bool, error)
	ListByBlocker(blockerID string, limit, offset int) ([]*model.Blocking, error)
}

type blockingRepository struct {
	db *gorm.DB
}

// NewBlockingRepository creates a new BlockingRepository.
func NewBlockingRepository(db *gorm.DB) BlockingRepository {
	return &blockingRepository{db: db}
}

func (r *blockingRepository) Create(b *model.Blocking) error {
	return r.db.Create(b).Error
}

func (r *blockingRepository) Delete(b *model.Blocking) error {
	return r.db.Delete(b).Error
}

func (r *blockingRepository) FindByPair(blockerID, blockeeID string) (*model.Blocking, error) {
	var b model.Blocking
	if err := r.db.Where("\"blockerId\" = ? AND \"blockeeId\" = ?", blockerID, blockeeID).First(&b).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *blockingRepository) Exists(blockerID, blockeeID string) (bool, error) {
	var count int64
	if err := r.db.Model(&model.Blocking{}).
		Where("\"blockerId\" = ? AND \"blockeeId\" = ?", blockerID, blockeeID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *blockingRepository) ListByBlocker(blockerID string, limit, offset int) ([]*model.Blocking, error) {
	var rows []*model.Blocking
	if err := r.db.Where("\"blockerId\" = ?", blockerID).
		Order("id DESC").
		Limit(limit).
		Offset(offset).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
