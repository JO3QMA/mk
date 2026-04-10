package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// ReversiRepository handles reversi_game persistence.
type ReversiRepository interface {
	Create(game *model.ReversiGame) error
	FindByID(id string) (*model.ReversiGame, error)
	Update(game *model.ReversiGame) error
	ListByUser(userID string, limit int) ([]*model.ReversiGame, error)
	ListActive() ([]*model.ReversiGame, error)
	Delete(id string) error
}

type reversiRepository struct {
	db *gorm.DB
}

// NewReversiRepository creates a new ReversiRepository.
func NewReversiRepository(db *gorm.DB) ReversiRepository {
	return &reversiRepository{db: db}
}

func (r *reversiRepository) Create(game *model.ReversiGame) error {
	return r.db.Create(game).Error
}

func (r *reversiRepository) FindByID(id string) (*model.ReversiGame, error) {
	var game model.ReversiGame
	if err := r.db.Preload("User1").Preload("User2").Where(`"id" = ?`, id).First(&game).Error; err != nil {
		return nil, err
	}
	return &game, nil
}

func (r *reversiRepository) Update(game *model.ReversiGame) error {
	return r.db.Save(game).Error
}

func (r *reversiRepository) ListByUser(userID string, limit int) ([]*model.ReversiGame, error) {
	if limit <= 0 {
		limit = 10
	}
	var games []*model.ReversiGame
	if err := r.db.Preload("User1").Preload("User2").
		Where(`"user1Id" = ? OR "user2Id" = ?`, userID, userID).
		Order(`"id" DESC`).Limit(limit).Find(&games).Error; err != nil {
		return nil, err
	}
	return games, nil
}

func (r *reversiRepository) ListActive() ([]*model.ReversiGame, error) {
	var games []*model.ReversiGame
	if err := r.db.Preload("User1").Preload("User2").
		Where(`"isEnded" = false`).
		Order(`"id" DESC`).Limit(50).Find(&games).Error; err != nil {
		return nil, err
	}
	return games, nil
}

func (r *reversiRepository) Delete(id string) error {
	return r.db.Where(`"id" = ?`, id).Delete(&model.ReversiGame{}).Error
}
