package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UserPublickeyRepository provides data access for remote user public keys.
type UserPublickeyRepository interface {
	Upsert(pk *model.UserPublickey) error
	FindByUserID(userID string) (*model.UserPublickey, error)
	FindByKeyID(keyID string) (*model.UserPublickey, error)
	Delete(userID string) error
}

type userPublickeyRepository struct {
	db *gorm.DB
}

// NewUserPublickeyRepository creates a new UserPublickeyRepository.
func NewUserPublickeyRepository(db *gorm.DB) UserPublickeyRepository {
	return &userPublickeyRepository{db: db}
}

func (r *userPublickeyRepository) Upsert(pk *model.UserPublickey) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "userId"}},
		DoUpdates: clause.AssignmentColumns([]string{"keyId", "keyPem"}),
	}).Create(pk).Error
}

func (r *userPublickeyRepository) FindByUserID(userID string) (*model.UserPublickey, error) {
	var pk model.UserPublickey
	if err := r.db.Where(`"userId" = ?`, userID).First(&pk).Error; err != nil {
		return nil, err
	}
	return &pk, nil
}

func (r *userPublickeyRepository) FindByKeyID(keyID string) (*model.UserPublickey, error) {
	var pk model.UserPublickey
	if err := r.db.Where(`"keyId" = ?`, keyID).First(&pk).Error; err != nil {
		return nil, err
	}
	return &pk, nil
}

func (r *userPublickeyRepository) Delete(userID string) error {
	return r.db.Where(`"userId" = ?`, userID).Delete(&model.UserPublickey{}).Error
}
