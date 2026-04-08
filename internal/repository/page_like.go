package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// PageLikeRepository provides data access for the `page_like` table.
type PageLikeRepository interface {
	Create(l *model.PageLike) error
	Delete(l *model.PageLike) error
	FindByPair(userID, pageID string) (*model.PageLike, error)
	Exists(userID, pageID string) (bool, error)
}

type pageLikeRepository struct {
	db *gorm.DB
}

// NewPageLikeRepository creates a new PageLikeRepository.
func NewPageLikeRepository(db *gorm.DB) PageLikeRepository {
	return &pageLikeRepository{db: db}
}

func (r *pageLikeRepository) Create(l *model.PageLike) error {
	return r.db.Create(l).Error
}

func (r *pageLikeRepository) Delete(l *model.PageLike) error {
	return r.db.Delete(l).Error
}

func (r *pageLikeRepository) FindByPair(userID, pageID string) (*model.PageLike, error) {
	var l model.PageLike
	if err := r.db.Where("\"userId\" = ? AND \"pageId\" = ?", userID, pageID).First(&l).Error; err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *pageLikeRepository) Exists(userID, pageID string) (bool, error) {
	var count int64
	if err := r.db.Model(&model.PageLike{}).
		Where("\"userId\" = ? AND \"pageId\" = ?", userID, pageID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
