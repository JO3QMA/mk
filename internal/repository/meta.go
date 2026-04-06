package repository

import (
	"github.com/misskey-dev/misskey-go/internal/model"
	"gorm.io/gorm"
)

// MetaRepository provides data access for server metadata.
type MetaRepository interface {
	Fetch() (*model.Meta, error)
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
