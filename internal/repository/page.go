package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// PageRepository provides data access for the `page` table.
type PageRepository interface {
	Create(p *model.Page) error
	FindByID(id string) (*model.Page, error)
	FindByUserAndName(userID, name string) (*model.Page, error)
	UpdateFields(pageID string, fields map[string]any) error
	Delete(p *model.Page) error
	ListByUser(userID string, limit, offset int) ([]*model.Page, error)
	// ListPublicByUser returns only public pages owned by userID, used by
	// users/pages when viewer is not the owner.
	ListPublicByUser(userID string, limit, offset int) ([]*model.Page, error)
	ListFeatured(limit, offset int) ([]*model.Page, error)
	IncrementCount(pageID, column string, delta int) error
}

type pageRepository struct {
	db *gorm.DB
}

// NewPageRepository creates a new PageRepository.
func NewPageRepository(db *gorm.DB) PageRepository {
	return &pageRepository{db: db}
}

func (r *pageRepository) Create(p *model.Page) error {
	return r.db.Create(p).Error
}

func (r *pageRepository) FindByID(id string) (*model.Page, error) {
	var p model.Page
	if err := r.db.First(&p, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// FindByUserAndName looks up a page by the (userId, name) pair which is the
// primary user-facing identity for a Page in Misskey.
func (r *pageRepository) FindByUserAndName(userID, name string) (*model.Page, error) {
	var p model.Page
	if err := r.db.Where("\"userId\" = ? AND name = ?", userID, name).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *pageRepository) UpdateFields(pageID string, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	return r.db.Model(&model.Page{}).Where("id = ?", pageID).Updates(fields).Error
}

func (r *pageRepository) Delete(p *model.Page) error {
	return r.db.Delete(p).Error
}

// ListByUser returns the pages owned by userID, ordered by updatedAt desc.
func (r *pageRepository) ListByUser(userID string, limit, offset int) ([]*model.Page, error) {
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	var rows []*model.Page
	if err := r.db.Where("\"userId\" = ?", userID).
		Order("\"updatedAt\" DESC").
		Limit(limit).
		Offset(offset).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *pageRepository) ListPublicByUser(userID string, limit, offset int) ([]*model.Page, error) {
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	var rows []*model.Page
	if err := r.db.Where("\"userId\" = ? AND visibility = ?", userID, string(model.PageVisibilityPublic)).
		Order("\"updatedAt\" DESC").
		Limit(limit).
		Offset(offset).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListFeatured returns the most-liked public pages.
func (r *pageRepository) ListFeatured(limit, offset int) ([]*model.Page, error) {
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	var rows []*model.Page
	if err := r.db.Where("visibility = ?", string(model.PageVisibilityPublic)).
		Order("\"likedCount\" DESC").
		Limit(limit).
		Offset(offset).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// IncrementCount adjusts a counter column on the page row by delta.
func (r *pageRepository) IncrementCount(pageID, column string, delta int) error {
	return r.db.Model(&model.Page{}).
		Where("id = ?", pageID).
		UpdateColumn(column, gorm.Expr("\""+column+"\" + ?", delta)).Error
}
