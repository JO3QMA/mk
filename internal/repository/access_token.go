package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// AccessTokenRepository provides data access for access tokens.
type AccessTokenRepository interface {
	FindByHash(hash string) (*model.AccessToken, error)
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
