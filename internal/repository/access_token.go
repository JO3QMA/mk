package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// AccessTokenRepository provides data access for access tokens.
type AccessTokenRepository interface {
	FindByHash(hash string) (*model.AccessToken, error)
	FindByID(id string) (*model.AccessToken, error)
	ListByUserID(userID string) ([]*model.AccessToken, error)
	DeleteByID(id string) error
}

type accessTokenRepository struct {
	db *gorm.DB
}

// NewAccessTokenRepository creates a new AccessTokenRepository.
func NewAccessTokenRepository(db *gorm.DB) AccessTokenRepository {
	return &accessTokenRepository{db: db}
}

func (r *accessTokenRepository) FindByHash(hash string) (*model.AccessToken, error) {
	var token model.AccessToken
	if err := r.db.Where("hash = ?", hash).Preload("User").First(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

func (r *accessTokenRepository) FindByID(id string) (*model.AccessToken, error) {
	var token model.AccessToken
	if err := r.db.Where(`"id" = ?`, id).First(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

// ListByUserID returns all access tokens owned by the given user. Used
// by i/authorized-apps; ordering mirrors upstream (newest first).
func (r *accessTokenRepository) ListByUserID(userID string) ([]*model.AccessToken, error) {
	var tokens []*model.AccessToken
	if err := r.db.Where(`"userId" = ?`, userID).Order(`"id" DESC`).Find(&tokens).Error; err != nil {
		return nil, err
	}
	return tokens, nil
}

// DeleteByID removes an access token. i/revoke-token の権限チェックは呼び出し
// 側 (handler) の責務で、ここでは id 一致のみ扱う。
func (r *accessTokenRepository) DeleteByID(id string) error {
	return r.db.Where(`"id" = ?`, id).Delete(&model.AccessToken{}).Error
}
