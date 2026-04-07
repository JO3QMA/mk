package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// EmojiRepository provides data access for the `emoji` table.
type EmojiRepository interface {
	FindByNameAndHost(name string, host *string) (*model.Emoji, error)
}

type emojiRepository struct {
	db *gorm.DB
}

// NewEmojiRepository creates a new EmojiRepository.
func NewEmojiRepository(db *gorm.DB) EmojiRepository {
	return &emojiRepository{db: db}
}

// FindByNameAndHost looks up a custom emoji by its name and host.
// host=nil の場合はローカル絵文字 (host IS NULL) として検索する。
func (r *emojiRepository) FindByNameAndHost(name string, host *string) (*model.Emoji, error) {
	var e model.Emoji
	q := r.db.Where("name = ?", name)
	if host == nil {
		q = q.Where("host IS NULL")
	} else {
		q = q.Where("host = ?", *host)
	}
	if err := q.First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}
