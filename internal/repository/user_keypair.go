package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// UserKeypairRepository provides data access for the `user_keypair` table.
type UserKeypairRepository interface {
	Create(k *model.UserKeypair) error
	FindByUserID(userID string) (*model.UserKeypair, error)
}

type userKeypairRepository struct {
	db *gorm.DB
}

// NewUserKeypairRepository creates a new UserKeypairRepository.
func NewUserKeypairRepository(db *gorm.DB) UserKeypairRepository {
	return &userKeypairRepository{db: db}
}

func (r *userKeypairRepository) Create(k *model.UserKeypair) error {
	return r.db.Create(k).Error
}

func (r *userKeypairRepository) FindByUserID(userID string) (*model.UserKeypair, error) {
	var k model.UserKeypair
	if err := r.db.First(&k, "\"userId\" = ?", userID).Error; err != nil {
		return nil, err
	}
	return &k, nil
}
