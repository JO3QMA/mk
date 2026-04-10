package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// SwSubscriptionRepository handles sw_subscription persistence.
type SwSubscriptionRepository interface {
	FindByUserAndEndpoint(userID, endpoint string) (*model.SwSubscription, error)
	Create(sub *model.SwSubscription) error
	Update(sub *model.SwSubscription) error
	DeleteByEndpoint(endpoint string) error
	DeleteByUserAndEndpoint(userID, endpoint string) error
}

type swSubscriptionRepository struct {
	db *gorm.DB
}

// NewSwSubscriptionRepository creates a new SwSubscriptionRepository.
func NewSwSubscriptionRepository(db *gorm.DB) SwSubscriptionRepository {
	return &swSubscriptionRepository{db: db}
}

func (r *swSubscriptionRepository) FindByUserAndEndpoint(userID, endpoint string) (*model.SwSubscription, error) {
	var sub model.SwSubscription
	if err := r.db.Where(`"userId" = ? AND "endpoint" = ?`, userID, endpoint).First(&sub).Error; err != nil {
		return nil, err
	}
	return &sub, nil
}

func (r *swSubscriptionRepository) Create(sub *model.SwSubscription) error {
	return r.db.Create(sub).Error
}

func (r *swSubscriptionRepository) Update(sub *model.SwSubscription) error {
	return r.db.Save(sub).Error
}

func (r *swSubscriptionRepository) DeleteByEndpoint(endpoint string) error {
	return r.db.Where(`"endpoint" = ?`, endpoint).Delete(&model.SwSubscription{}).Error
}

func (r *swSubscriptionRepository) DeleteByUserAndEndpoint(userID, endpoint string) error {
	return r.db.Where(`"userId" = ? AND "endpoint" = ?`, userID, endpoint).Delete(&model.SwSubscription{}).Error
}
