package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// ChannelFollowingRepository provides data access for the
// `channel_following` table.
type ChannelFollowingRepository interface {
	Create(f *model.ChannelFollowing) error
	Delete(f *model.ChannelFollowing) error
	FindByPair(followerID, channelID string) (*model.ChannelFollowing, error)
	Exists(followerID, channelID string) (bool, error)
	ListFollowed(userID string, limit, offset int) ([]*model.ChannelFollowing, error)
}

type channelFollowingRepository struct {
	db *gorm.DB
}

// NewChannelFollowingRepository creates a new ChannelFollowingRepository.
func NewChannelFollowingRepository(db *gorm.DB) ChannelFollowingRepository {
	return &channelFollowingRepository{db: db}
}

func (r *channelFollowingRepository) Create(f *model.ChannelFollowing) error {
	return r.db.Create(f).Error
}

func (r *channelFollowingRepository) Delete(f *model.ChannelFollowing) error {
	return r.db.Delete(f).Error
}

func (r *channelFollowingRepository) FindByPair(followerID, channelID string) (*model.ChannelFollowing, error) {
	var f model.ChannelFollowing
	if err := r.db.Where("\"followerId\" = ? AND \"followeeId\" = ?", followerID, channelID).First(&f).Error; err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *channelFollowingRepository) Exists(followerID, channelID string) (bool, error) {
	var count int64
	if err := r.db.Model(&model.ChannelFollowing{}).
		Where("\"followerId\" = ? AND \"followeeId\" = ?", followerID, channelID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// ListFollowed returns the channel followings for a given user, ordered by id
// descending (newest first).
func (r *channelFollowingRepository) ListFollowed(userID string, limit, offset int) ([]*model.ChannelFollowing, error) {
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	var rows []*model.ChannelFollowing
	if err := r.db.Where("\"followerId\" = ?", userID).
		Order("id DESC").
		Limit(limit).
		Offset(offset).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
