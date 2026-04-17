package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// FlashRepository provides data access for the `flash` table.
type FlashRepository interface {
	Create(f *model.Flash) error
	FindByID(id string) (*model.Flash, error)
	UpdateFields(flashID string, fields map[string]any) error
	Delete(f *model.Flash) error
	ListByUser(userID string, limit, offset int) ([]*model.Flash, error)
	// ListPublicByUser returns only public flashes (visibility='public') owned
	// by userID, used by users/flashs when viewer is not the owner.
	ListPublicByUser(userID string, limit, offset int) ([]*model.Flash, error)
	ListFeatured(limit, offset int) ([]*model.Flash, error)
	Search(query string, limit, offset int) ([]*model.Flash, error)
	IncrementCount(flashID, column string, delta int) error
}

type flashRepository struct {
	db *gorm.DB
}

// NewFlashRepository creates a new FlashRepository.
func NewFlashRepository(db *gorm.DB) FlashRepository {
	return &flashRepository{db: db}
}

func (r *flashRepository) Create(f *model.Flash) error {
	return r.db.Create(f).Error
}

func (r *flashRepository) FindByID(id string) (*model.Flash, error) {
	var f model.Flash
	if err := r.db.First(&f, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *flashRepository) UpdateFields(flashID string, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	return r.db.Model(&model.Flash{}).Where("id = ?", flashID).Updates(fields).Error
}

func (r *flashRepository) Delete(f *model.Flash) error {
	return r.db.Delete(f).Error
}

// ListByUser returns the flashes owned by userID, ordered by updatedAt desc.
func (r *flashRepository) ListByUser(userID string, limit, offset int) ([]*model.Flash, error) {
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	var rows []*model.Flash
	if err := r.db.Where("\"userId\" = ?", userID).
		Order("\"updatedAt\" DESC").
		Limit(limit).
		Offset(offset).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *flashRepository) ListPublicByUser(userID string, limit, offset int) ([]*model.Flash, error) {
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	var rows []*model.Flash
	if err := r.db.Where("\"userId\" = ? AND visibility = 'public'", userID).
		Order("\"updatedAt\" DESC").
		Limit(limit).
		Offset(offset).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListFeatured returns the most-liked flashes overall.
func (r *flashRepository) ListFeatured(limit, offset int) ([]*model.Flash, error) {
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	var rows []*model.Flash
	if err := r.db.
		Order("\"likedCount\" DESC").
		Limit(limit).
		Offset(offset).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// Search performs a substring search across title and summary, ordered by
// updatedAt desc.
func (r *flashRepository) Search(query string, limit, offset int) ([]*model.Flash, error) {
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	var rows []*model.Flash
	if err := r.db.
		Where("title ILIKE ? OR summary ILIKE ?", "%"+query+"%", "%"+query+"%").
		Order("\"updatedAt\" DESC").
		Limit(limit).
		Offset(offset).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// IncrementCount adjusts a counter column on the flash row by delta.
func (r *flashRepository) IncrementCount(flashID, column string, delta int) error {
	return r.db.Model(&model.Flash{}).
		Where("id = ?", flashID).
		UpdateColumn(column, gorm.Expr("\""+column+"\" + ?", delta)).Error
}
