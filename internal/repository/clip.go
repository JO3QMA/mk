package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// ClipRepository provides data access for the `clip` table.
type ClipRepository interface {
	Create(c *model.Clip) error
	FindByID(id string) (*model.Clip, error)
	UpdateFields(clipID string, fields map[string]any) error
	Delete(c *model.Clip) error
	ListByUser(userID string, limit, offset int) ([]*model.Clip, error)
	// ListPublicByUser returns only public clips owned by userID. Used by
	// users/clips when the viewer is not the owner so LIMIT applies to the
	// already-filtered set.
	ListPublicByUser(userID string, limit, offset int) ([]*model.Clip, error)
	IncrementCount(clipID, column string, delta int) error
}

type clipRepository struct {
	db *gorm.DB
}

// NewClipRepository creates a new ClipRepository.
func NewClipRepository(db *gorm.DB) ClipRepository {
	return &clipRepository{db: db}
}

func (r *clipRepository) Create(c *model.Clip) error {
	return r.db.Create(c).Error
}

func (r *clipRepository) FindByID(id string) (*model.Clip, error) {
	var c model.Clip
	if err := r.db.First(&c, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *clipRepository) UpdateFields(clipID string, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	return r.db.Model(&model.Clip{}).Where("id = ?", clipID).Updates(fields).Error
}

func (r *clipRepository) Delete(c *model.Clip) error {
	return r.db.Delete(c).Error
}

// ListByUser returns clips owned by userID, ordered by id desc.
func (r *clipRepository) ListByUser(userID string, limit, offset int) ([]*model.Clip, error) {
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	var rows []*model.Clip
	if err := r.db.Where("\"userId\" = ?", userID).
		Order("id DESC").
		Limit(limit).
		Offset(offset).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *clipRepository) ListPublicByUser(userID string, limit, offset int) ([]*model.Clip, error) {
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	var rows []*model.Clip
	if err := r.db.Where("\"userId\" = ? AND \"isPublic\" = true", userID).
		Order("id DESC").
		Limit(limit).
		Offset(offset).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// IncrementCount adjusts a counter column on the clip row by delta.
func (r *clipRepository) IncrementCount(clipID, column string, delta int) error {
	return r.db.Model(&model.Clip{}).
		Where("id = ?", clipID).
		UpdateColumn(column, gorm.Expr("\""+column+"\" + ?", delta)).Error
}
