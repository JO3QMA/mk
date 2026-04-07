package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// UserNotePiningRepository provides data access for the `user_note_pining` table.
type UserNotePiningRepository interface {
	Create(p *model.UserNotePining) error
	Delete(p *model.UserNotePining) error
	FindByPair(userID, noteID string) (*model.UserNotePining, error)
	ListByUser(userID string) ([]*model.UserNotePining, error)
	CountByUser(userID string) (int, error)
}

type userNotePiningRepository struct {
	db *gorm.DB
}

// NewUserNotePiningRepository creates a new UserNotePiningRepository.
func NewUserNotePiningRepository(db *gorm.DB) UserNotePiningRepository {
	return &userNotePiningRepository{db: db}
}

func (r *userNotePiningRepository) Create(p *model.UserNotePining) error {
	return r.db.Create(p).Error
}

func (r *userNotePiningRepository) Delete(p *model.UserNotePining) error {
	return r.db.Delete(p).Error
}

func (r *userNotePiningRepository) FindByPair(userID, noteID string) (*model.UserNotePining, error) {
	var p model.UserNotePining
	if err := r.db.Where("\"userId\" = ? AND \"noteId\" = ?", userID, noteID).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *userNotePiningRepository) ListByUser(userID string) ([]*model.UserNotePining, error) {
	var rows []*model.UserNotePining
	if err := r.db.Where("\"userId\" = ?", userID).
		Order("id DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *userNotePiningRepository) CountByUser(userID string) (int, error) {
	var count int64
	if err := r.db.Model(&model.UserNotePining{}).
		Where("\"userId\" = ?", userID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}
