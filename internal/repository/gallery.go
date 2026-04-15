package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// GalleryRepository provides data access for gallery posts and likes
// scoped to a single user. i/gallery/posts と i/gallery/likes の
// per-user 一覧で利用する。
type GalleryRepository interface {
	ListByUser(userID string, limit, offset int) ([]*model.GalleryPost, error)
	ListLikesByUser(userID string, limit, offset int) ([]*model.GalleryLike, error)
}

type galleryRepository struct {
	db *gorm.DB
}

// NewGalleryRepository builds a GalleryRepository backed by gorm.
func NewGalleryRepository(db *gorm.DB) GalleryRepository {
	return &galleryRepository{db: db}
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return 10
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func (r *galleryRepository) ListByUser(userID string, limit, offset int) ([]*model.GalleryPost, error) {
	limit = clampLimit(limit)
	q := r.db.Where(`"userId" = ?`, userID).Order(`"id" DESC`).Limit(limit)
	if offset > 0 {
		q = q.Offset(offset)
	}
	var posts []*model.GalleryPost
	if err := q.Find(&posts).Error; err != nil {
		return nil, err
	}
	return posts, nil
}

func (r *galleryRepository) ListLikesByUser(userID string, limit, offset int) ([]*model.GalleryLike, error) {
	limit = clampLimit(limit)
	q := r.db.Where(`"userId" = ?`, userID).Order(`"id" DESC`).Limit(limit)
	if offset > 0 {
		q = q.Offset(offset)
	}
	var likes []*model.GalleryLike
	if err := q.Find(&likes).Error; err != nil {
		return nil, err
	}
	return likes, nil
}
