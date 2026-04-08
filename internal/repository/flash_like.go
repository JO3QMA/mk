package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// FlashLikeRepository provides data access for the `flash_like` table.
type FlashLikeRepository interface {
	Create(l *model.FlashLike) error
	Delete(l *model.FlashLike) error
	FindByPair(userID, flashID string) (*model.FlashLike, error)
	Exists(userID, flashID string) (bool, error)
	ListByUser(userID string, limit, offset int) ([]*model.FlashLike, error)
}

type flashLikeRepository struct {
	db *gorm.DB
}

// NewFlashLikeRepository creates a new FlashLikeRepository.
func NewFlashLikeRepository(db *gorm.DB) FlashLikeRepository {
	return &flashLikeRepository{db: db}
}

func (r *flashLikeRepository) Create(l *model.FlashLike) error {
	return r.db.Create(l).Error
}

func (r *flashLikeRepository) Delete(l *model.FlashLike) error {
	return r.db.Delete(l).Error
}

func (r *flashLikeRepository) FindByPair(userID, flashID string) (*model.FlashLike, error) {
	var l model.FlashLike
	if err := r.db.Where("\"userId\" = ? AND \"flashId\" = ?", userID, flashID).First(&l).Error; err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *flashLikeRepository) Exists(userID, flashID string) (bool, error) {
	var count int64
	if err := r.db.Model(&model.FlashLike{}).
		Where("\"userId\" = ? AND \"flashId\" = ?", userID, flashID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// ListByUser returns the flash_like rows owned by userID newest first
// (id desc), used by Misskey 互換 i/flashs/likes endpoint.
func (r *flashLikeRepository) ListByUser(userID string, limit, offset int) ([]*model.FlashLike, error) {
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	var rows []*model.FlashLike
	if err := r.db.Where("\"userId\" = ?", userID).
		Order("id DESC").
		Limit(limit).
		Offset(offset).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
