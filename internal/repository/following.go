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
	// ListRemoteFollowerInboxes returns the deduplicated list of inbox URLs for
	// remote followers of userID. sharedInbox を持つフォロワーは sharedInbox
	// に集約され、無いフォロワーは個別inboxを返す。
	ListRemoteFollowerInboxes(userID string) ([]string, error)
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

// ListRemoteFollowerInboxes returns deduplicated inbox URLs for remote
// followers. sharedInbox を優先し、無い場合のみ個別 inbox を使う。
//
// SQLは以下の流れ:
//  1. follower がリモート (host IS NOT NULL) のフォロワーをjoin
//  2. sharedInbox があれば sharedInbox、無ければ inbox を選択
//  3. NULL/空文字列を除外し DISTINCT で重複排除
func (r *followingRepository) ListRemoteFollowerInboxes(userID string) ([]string, error) {
	var inboxes []string
	err := r.db.
		Table(`"following" AS f`).
		Select(`DISTINCT COALESCE(NULLIF(u."sharedInbox", ''), u.inbox) AS inbox`).
		Joins(`JOIN "user" u ON u.id = f."followerId"`).
		Where(`f."followeeId" = ? AND u.host IS NOT NULL AND (u."sharedInbox" IS NOT NULL OR u.inbox IS NOT NULL)`, userID).
		Pluck("inbox", &inboxes).Error
	if err != nil {
		return nil, err
	}
	// COALESCE が NULL を返す可能性があるため、空文字を除去
	out := make([]string, 0, len(inboxes))
	for _, inb := range inboxes {
		if inb != "" {
			out = append(out, inb)
		}
	}
	return out, nil
}
