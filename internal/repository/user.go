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
