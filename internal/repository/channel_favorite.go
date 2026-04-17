package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// ChannelFavoriteRepository provides data access for channel favorites.
type ChannelFavoriteRepository interface {
	Create(fav *model.ChannelFavorite) error
	Delete(userID, channelID string) error
	ListByUser(userID string) ([]*model.ChannelFavorite, error)
	Exists(userID, channelID string) (bool, error)
}

type channelFavoriteRepository struct {
	db *gorm.DB
}

// NewChannelFavoriteRepository creates a new ChannelFavoriteRepository.
func NewChannelFavoriteRepository(db *gorm.DB) ChannelFavoriteRepository {
	return &channelFavoriteRepository{db: db}
}

func (r *channelFavoriteRepository) Create(fav *model.ChannelFavorite) error {
	return r.db.Create(fav).Error
}

func (r *channelFavoriteRepository) Delete(userID, channelID string) error {
	return r.db.Where(`"userId" = ? AND "channelId" = ?`, userID, channelID).
		Delete(&model.ChannelFavorite{}).Error
}

func (r *channelFavoriteRepository) ListByUser(userID string) ([]*model.ChannelFavorite, error) {
	var favs []*model.ChannelFavorite
	if err := r.db.Where(`"userId" = ?`, userID).Find(&favs).Error; err != nil {
		return nil, err
	}
	return favs, nil
}

func (r *channelFavoriteRepository) Exists(userID, channelID string) (bool, error) {
	var count int64
	err := r.db.Model(&model.ChannelFavorite{}).
		Where(`"userId" = ? AND "channelId" = ?`, userID, channelID).Count(&count).Error
	return count > 0, err
}
