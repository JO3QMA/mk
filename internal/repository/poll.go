package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// PollRepository provides data access for polls.
type PollRepository interface {
	Create(poll *model.Poll) error
}

type pollRepository struct {
	db *gorm.DB
}

// NewPollRepository creates a new PollRepository.
func NewPollRepository(db *gorm.DB) PollRepository {
	return &pollRepository{db: db}
}

func (r *pollRepository) Create(poll *model.Poll) error {
	return r.db.Create(poll).Error
}
