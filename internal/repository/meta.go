package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// MetaRepository provides data access for server metadata.
type MetaRepository interface {
	Fetch() (*model.Meta, error)
	Update(fields map[string]any) error
	EnsureInitial(id string) error
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

// EnsureInitial creates the singleton meta row if it does not exist yet.
// Called once at server startup so that fresh installs can proceed with
// /api/admin/accounts/create (initial setup) without hitting a 500.
func (r *metaRepository) EnsureInitial(id string) error {
	var meta model.Meta
	err := r.db.First(&meta).Error
	if err == nil {
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	return r.db.Create(&model.Meta{ID: id}).Error
}
