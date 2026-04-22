package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// FollowRequestRepository provides data access for the `follow_request` table.
type FollowRequestRepository interface {
	Create(r *model.FollowRequest) error
	Delete(r *model.FollowRequest) error
	FindByPair(followerID, followeeID string) (*model.FollowRequest, error)
	Exists(followerID, followeeID string) (bool, error)
	// ListReceived returns follow requests where userID is the followee.
	// sinceID / untilID はMisskey本家の pagination cursor。空文字で無指定扱い。
	ListReceived(userID string, limit int, sinceID, untilID string) ([]*model.FollowRequest, error)
	// ListSent returns follow requests where userID is the follower.
	ListSent(userID string, limit int, sinceID, untilID string) ([]*model.FollowRequest, error)
	// CountReceived returns the number of pending follow requests received by
	// userID. Used by /api/i to compute hasPendingReceivedFollowRequest.
	CountReceived(userID string) (int64, error)
}

type followRequestRepository struct {
	db *gorm.DB
}

// NewFollowRequestRepository creates a new FollowRequestRepository.
func NewFollowRequestRepository(db *gorm.DB) FollowRequestRepository {
	return &followRequestRepository{db: db}
}

func (r *followRequestRepository) Create(req *model.FollowRequest) error {
	return r.db.Create(req).Error
}

func (r *followRequestRepository) Delete(req *model.FollowRequest) error {
	return r.db.Delete(req).Error
}

func (r *followRequestRepository) FindByPair(followerID, followeeID string) (*model.FollowRequest, error) {
	var req model.FollowRequest
	if err := r.db.Where("\"followerId\" = ? AND \"followeeId\" = ?", followerID, followeeID).First(&req).Error; err != nil {
		return nil, err
	}
	return &req, nil
}

func (r *followRequestRepository) Exists(followerID, followeeID string) (bool, error) {
	var count int64
	if err := r.db.Model(&model.FollowRequest{}).
		Where("\"followerId\" = ? AND \"followeeId\" = ?", followerID, followeeID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *followRequestRepository) ListReceived(userID string, limit int, sinceID, untilID string) ([]*model.FollowRequest, error) {
	return r.listByColumn("\"followeeId\"", userID, limit, sinceID, untilID)
}

func (r *followRequestRepository) CountReceived(userID string) (int64, error) {
	var count int64
	if err := r.db.Model(&model.FollowRequest{}).
		Where("\"followeeId\" = ?", userID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *followRequestRepository) ListSent(userID string, limit int, sinceID, untilID string) ([]*model.FollowRequest, error) {
	return r.listByColumn("\"followerId\"", userID, limit, sinceID, untilID)
}

// listByColumn is the shared query builder for ListReceived / ListSent.
func (r *followRequestRepository) listByColumn(col, userID string, limit int, sinceID, untilID string) ([]*model.FollowRequest, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	q := r.db.Where(col+" = ?", userID).Order(paginationOrder(sinceID, untilID, "id")).Limit(limit)
	if sinceID != "" {
		q = q.Where("id > ?", sinceID)
	}
	if untilID != "" {
		q = q.Where("id < ?", untilID)
	}
	var rows []*model.FollowRequest
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
