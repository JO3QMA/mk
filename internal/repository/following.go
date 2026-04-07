package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// FollowingRepository provides data access for the `following` table.
type FollowingRepository interface {
	Create(f *model.Following) error
	Delete(f *model.Following) error
	FindByPair(followerID, followeeID string) (*model.Following, error)
	Exists(followerID, followeeID string) (bool, error)
	ListFollowers(userID string, limit, offset int) ([]*model.Following, error)
	ListFollowing(userID string, limit, offset int) ([]*model.Following, error)
}

type followingRepository struct {
	db *gorm.DB
}

// NewFollowingRepository creates a new FollowingRepository.
func NewFollowingRepository(db *gorm.DB) FollowingRepository {
	return &followingRepository{db: db}
}

func (r *followingRepository) Create(f *model.Following) error {
	return r.db.Create(f).Error
}

func (r *followingRepository) Delete(f *model.Following) error {
	return r.db.Delete(f).Error
}

func (r *followingRepository) FindByPair(followerID, followeeID string) (*model.Following, error) {
	var f model.Following
	if err := r.db.Where("\"followerId\" = ? AND \"followeeId\" = ?", followerID, followeeID).First(&f).Error; err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *followingRepository) Exists(followerID, followeeID string) (bool, error) {
	var count int64
	if err := r.db.Model(&model.Following{}).
		Where("\"followerId\" = ? AND \"followeeId\" = ?", followerID, followeeID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *followingRepository) ListFollowers(userID string, limit, offset int) ([]*model.Following, error) {
	var rows []*model.Following
	if err := r.db.Where("\"followeeId\" = ?", userID).
		Order("id DESC").
		Limit(limit).
		Offset(offset).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *followingRepository) ListFollowing(userID string, limit, offset int) ([]*model.Following, error) {
	var rows []*model.Following
	if err := r.db.Where("\"followerId\" = ?", userID).
		Order("id DESC").
		Limit(limit).
		Offset(offset).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
