package repository

import (
	"time"

	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// Registration ticket filter values accepted by
// RegistrationTicketRepository.List. Declared as plain string constants so
// callers (admin handlers, testutil mocks) can pass literals without taking a
// dependency on this package's type system — avoiding an import cycle with
// testutil.
const (
	// RegistrationTicketAll returns every ticket.
	RegistrationTicketAll = "all"
	// RegistrationTicketUnused returns tickets that have not been redeemed.
	RegistrationTicketUnused = "unused"
	// RegistrationTicketUsed returns tickets that have been redeemed.
	RegistrationTicketUsed = "used"
	// RegistrationTicketExpired returns tickets past their expiresAt.
	RegistrationTicketExpired = "expired"
)

// RegistrationTicketRepository handles persistence for the
// `registration_ticket` table that backs admin invite code management.
type RegistrationTicketRepository interface {
	Create(t *model.RegistrationTicket) error
	// List returns tickets matching the filter, paginated with limit/offset.
	// Unknown filter values are treated as "all".
	// `now` is passed by callers so tests can supply a deterministic clock
	// when evaluating the `expired` filter.
	List(filter string, limit, offset int, now time.Time) ([]*model.RegistrationTicket, error)
	Delete(id string) error
}

type registrationTicketRepository struct {
	db *gorm.DB
}

// NewRegistrationTicketRepository returns a GORM-backed RegistrationTicketRepository.
func NewRegistrationTicketRepository(db *gorm.DB) RegistrationTicketRepository {
	return &registrationTicketRepository{db: db}
}

func (r *registrationTicketRepository) Create(t *model.RegistrationTicket) error {
	return r.db.Create(t).Error
}

func (r *registrationTicketRepository) List(filter string, limit, offset int, now time.Time) ([]*model.RegistrationTicket, error) {
	if limit <= 0 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}
	q := r.db.Model(&model.RegistrationTicket{})
	switch filter {
	case RegistrationTicketUnused:
		q = q.Where(`"usedById" IS NULL`)
	case RegistrationTicketUsed:
		q = q.Where(`"usedById" IS NOT NULL`)
	case RegistrationTicketExpired:
		q = q.Where(`"expiresAt" IS NOT NULL AND "expiresAt" < ?`, now)
	}
	var rows []*model.RegistrationTicket
	if err := q.Order(`"id" DESC`).Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *registrationTicketRepository) Delete(id string) error {
	return r.db.Where("id = ?", id).Delete(&model.RegistrationTicket{}).Error
}
