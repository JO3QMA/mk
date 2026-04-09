package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// MetaRepository provides data access for server metadata.
type MetaRepository interface {
	Fetch() (*model.Meta, error)
	Update(fields map[string]any) error
}

type metaRepository struct {
	db *gorm.DB
}

// NewMetaRepository creates a new MetaRepository.
func NewMetaRepository(db *gorm.DB) MetaRepository {
	return &metaRepository{db: db}
}

func (r *metaRepository) Fetch() (*model.Meta, error) {
	var meta model.Meta
	if err := r.db.First(&meta).Error; err != nil {
		return nil, err
	}
	return &meta, nil
}

func (r *metaRepository) Update(fields map[string]any) error {
	return r.db.Model(&model.Meta{}).Where("TRUE").Updates(fields).Error
}
