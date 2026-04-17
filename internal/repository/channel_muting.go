package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// ChannelMutingRepository provides data access for channel muting.
type ChannelMutingRepository interface {
	Create(m *model.ChannelMuting) error
	Delete(userID, channelID string) error
	ListByUser(userID string) ([]*model.ChannelMuting, error)
	Exists(userID, channelID string) (bool, error)
}

type channelMutingRepository struct {
	db *gorm.DB
}

// NewChannelMutingRepository creates a new ChannelMutingRepository.
func NewChannelMutingRepository(db *gorm.DB) ChannelMutingRepository {
	return &channelMutingRepository{db: db}
}

func (r *channelMutingRepository) Create(m *model.ChannelMuting) error {
	return r.db.Create(m).Error
}

func (r *channelMutingRepository) Delete(userID, channelID string) error {
	return r.db.Where(`"userId" = ? AND "channelId" = ?`, userID, channelID).
		Delete(&model.ChannelMuting{}).Error
}

func (r *channelMutingRepository) ListByUser(userID string) ([]*model.ChannelMuting, error) {
	var mutes []*model.ChannelMuting
	if err := r.db.Where(`"userId" = ?`, userID).Find(&mutes).Error; err != nil {
		return nil, err
	}
	return mutes, nil
}

func (r *channelMutingRepository) Exists(userID, channelID string) (bool, error) {
	var count int64
	err := r.db.Model(&model.ChannelMuting{}).
		Where(`"userId" = ? AND "channelId" = ?`, userID, channelID).Count(&count).Error
	return count > 0, err
}
