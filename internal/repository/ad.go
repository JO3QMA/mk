package repository

import (
	"time"

	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// AdRepository provides read access to the `ad` table for meta / timeline
// ad exposure. Writes currently live in admin/handler_stubs.go with direct
// DB access; this interface is scoped to the read side that the public
// /api/meta endpoint needs.
type AdRepository interface {
	// ListActive returns ads whose window contains `now` (startsAt <= now
	// < expiresAt). Ordering is by id DESC so newest ads come first, which
	// matches the admin list order.
	ListActive(now time.Time) ([]*model.Ad, error)
}

type adRepository struct {
	db *gorm.DB
}

// NewAdRepository constructs the default AdRepository.
func NewAdRepository(db *gorm.DB) AdRepository {
	return &adRepository{db: db}
}

func (r *adRepository) ListActive(now time.Time) ([]*model.Ad, error) {
	var ads []*model.Ad
	if err := r.db.
		Where(`"startsAt" <= ? AND "expiresAt" > ?`, now, now).
		Order(`id DESC`).
		Find(&ads).Error; err != nil {
		return nil, err
	}
	return ads, nil
}
