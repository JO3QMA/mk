package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// RetentionAggregationRepository provides data access for retention statistics.
type RetentionAggregationRepository interface {
	ListRecent(limit int) ([]*model.RetentionAggregation, error)
}

type retentionAggregationRepository struct {
	db *gorm.DB
}

// NewRetentionAggregationRepository creates a new RetentionAggregationRepository.
func NewRetentionAggregationRepository(db *gorm.DB) RetentionAggregationRepository {
	return &retentionAggregationRepository{db: db}
}

func (r *retentionAggregationRepository) ListRecent(limit int) ([]*model.RetentionAggregation, error) {
	var records []*model.RetentionAggregation
	if err := r.db.Order(`"id" DESC`).Limit(limit).Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}
