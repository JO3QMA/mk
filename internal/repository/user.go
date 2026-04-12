package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// UserRepository provides data access for users.
type UserRepository interface {
	Create(u *model.User) error
	FindByID(id string) (*model.User, error)
	FindByToken(token string) (*model.User, error)
	FindByURI(uri string) (*model.User, error)
	FindByUsernameLower(username string, host *string) (*model.User, error)
	FindProfileByUserID(userID string) (*model.UserProfile, error)
	IncrementFollowingCount(userID string, delta int) error
	IncrementFollowersCount(userID string, delta int) error
	SearchByUsername(query string, limit, offset int) ([]*model.User, error)
	UpdateUser(userID string, fields map[string]any) error
	UpdateProfile(userID string, fields map[string]any) error
	CreateProfile(profile *model.UserProfile) error
	ListUsers(filter model.UserListFilter) ([]*model.User, error)
	ListRemoteInboxes() ([]string, error)
	FindProfileByVerifyCode(code string) (*model.UserProfile, error)
}

type userRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new UserRepository.
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(u *model.User) error {
	return r.db.Create(u).Error
}

func (r *userRepository) FindByID(id string) (*model.User, error) {
	var user model.User
	if err := r.db.First(&user, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByURI(uri string) (*model.User, error) {
	var user model.User
	if err := r.db.Where("uri = ?", uri).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByToken(token string) (*model.User, error) {
	var user model.User
	if err := r.db.Where("token = ?", token).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByUsernameLower(username string, host *string) (*model.User, error) {
	var user model.User
	q := r.db.Where("\"usernameLower\" = lower(?)", username)
	if host != nil {
		q = q.Where("host = ?", *host)
	} else {
		q = q.Where("host IS NULL")
	}
	if err := q.First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindProfileByUserID(userID string) (*model.UserProfile, error) {
	var profile model.UserProfile
	if err := r.db.First(&profile, "\"userId\" = ?", userID).Error; err != nil {
		return nil, err
	}
	return &profile, nil
}

func (r *userRepository) IncrementFollowingCount(userID string, delta int) error {
	return r.db.Model(&model.User{}).
		Where("id = ?", userID).
		UpdateColumn("followingCount", gorm.Expr("\"followingCount\" + ?", delta)).Error
}

func (r *userRepository) IncrementFollowersCount(userID string, delta int) error {
	return r.db.Model(&model.User{}).
		Where("id = ?", userID).
		UpdateColumn("followersCount", gorm.Expr("\"followersCount\" + ?", delta)).Error
}

// SearchByUsername returns users whose usernameLower starts with the given query.
// Phase 4でMeilisearch統合予定だが、現状は単純なLIKE検索のみ。
func (r *userRepository) SearchByUsername(query string, limit, offset int) ([]*model.User, error) {
	var users []*model.User
	if err := r.db.
		Where("\"usernameLower\" LIKE ?", query+"%").
		Order("\"followersCount\" DESC, id ASC").
		Limit(limit).
		Offset(offset).
		Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// UpdateUser updates the given columns on the user table.
func (r *userRepository) UpdateUser(userID string, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	return r.db.Model(&model.User{}).Where("id = ?", userID).Updates(fields).Error
}

// UpdateProfile updates the given columns on the user_profile table.
func (r *userRepository) UpdateProfile(userID string, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	return r.db.Model(&model.UserProfile{}).Where("\"userId\" = ?", userID).Updates(fields).Error
}

// CreateProfile inserts a new user_profile row.
func (r *userRepository) CreateProfile(profile *model.UserProfile) error {
	return r.db.Create(profile).Error
}

// FindProfileByVerifyCode looks up a user_profile by emailVerifyCode.
func (r *userRepository) FindProfileByVerifyCode(code string) (*model.UserProfile, error) {
	var p model.UserProfile
	if err := r.db.Where(`"emailVerifyCode" = ?`, code).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// ListRemoteInboxes returns a deduplicated list of inbox URLs belonging to
// every known remote user. sharedInbox を優先し、無ければ個別 inbox を採用
// する。Public なアクティビティ (Delete 等) の broadcast に使う。
//
// SELECT DISTINCT で PostgreSQL 側 dedup させるのでリモートユーザー数が
// 数十万規模でも Go 側でマップを持たずに済む。空文字は NULLIF で NULL 化して
// WHERE inbox IS NOT NULL で除外している。
func (r *userRepository) ListRemoteInboxes() ([]string, error) {
	const query = `SELECT DISTINCT COALESCE(NULLIF("sharedInbox", ''), inbox) AS inbox
FROM "user"
WHERE host IS NOT NULL
  AND (COALESCE(NULLIF("sharedInbox", ''), inbox)) IS NOT NULL`
	var out []string
	if err := r.db.Raw(query).Scan(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// ListUsers returns users matching the filter.
func (r *userRepository) ListUsers(filter model.UserListFilter) ([]*model.User, error) {
	q := r.db.Model(&model.User{})

	switch filter.Origin {
	case "local":
		q = q.Where("host IS NULL")
	case "remote":
		q = q.Where("host IS NOT NULL")
	}

	switch filter.State {
	case "suspended":
		q = q.Where("\"isSuspended\" = true")
	case "alive":
		q = q.Where("\"isSuspended\" = false")
	}

	switch filter.Sort {
	case "+createdAt":
		q = q.Order("id ASC")
	case "-createdAt":
		q = q.Order("id DESC")
	case "+updatedAt":
		q = q.Order("\"updatedAt\" ASC NULLS LAST")
	case "-updatedAt":
		q = q.Order("\"updatedAt\" DESC NULLS LAST")
	case "+followersCount":
		q = q.Order("\"followersCount\" ASC")
	case "-followersCount":
		q = q.Order("\"followersCount\" DESC")
	default:
		q = q.Order("id DESC")
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	q = q.Limit(limit)
	if filter.Offset > 0 {
		q = q.Offset(filter.Offset)
	}

	var users []*model.User
	if err := q.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}
