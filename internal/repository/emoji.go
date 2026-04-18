package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// EmojiRepository provides data access for the `emoji` table.
type EmojiRepository interface {
	FindByNameAndHost(name string, host *string) (*model.Emoji, error)
	// ListLocal returns all local custom emojis (host IS NULL).
	ListLocal() ([]*model.Emoji, error)
	// Admin CRUD
	Create(e *model.Emoji) error
	FindByID(id string) (*model.Emoji, error)
	FindManyByIDs(ids []string) ([]*model.Emoji, error)
	UpdateFields(id string, fields map[string]any) error
	// UpdateFieldsMany applies the same field map to every row whose id is in
	// ids. Used by admin bulk ops (category / license / isSensitive etc.).
	UpdateFieldsMany(ids []string, fields map[string]any) error
	Delete(id string) error
	// DeleteMany removes rows matching any id in ids.
	DeleteMany(ids []string) error
	// ListWithFilter returns emojis matching search/category/host filters.
	ListWithFilter(query, category string, local bool, limit, offset int) ([]*model.Emoji, error)
	// ListRemoteWithFilter mirrors ListWithFilter for remote emojis. host empty
	// matches any remote host.
	ListRemoteWithFilter(query, host string, limit, offset int) ([]*model.Emoji, error)
}

type emojiRepository struct {
	db *gorm.DB
}

// NewEmojiRepository creates a new EmojiRepository.
func NewEmojiRepository(db *gorm.DB) EmojiRepository {
	return &emojiRepository{db: db}
}

func (r *emojiRepository) ListLocal() ([]*model.Emoji, error) {
	var emojis []*model.Emoji
	if err := r.db.Where("host IS NULL").Order("category ASC, name ASC").Find(&emojis).Error; err != nil {
		return nil, err
	}
	return emojis, nil
}

func (r *emojiRepository) Create(e *model.Emoji) error {
	return r.db.Create(e).Error
}

func (r *emojiRepository) FindByID(id string) (*model.Emoji, error) {
	var e model.Emoji
	if err := r.db.Where("id = ?", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *emojiRepository) UpdateFields(id string, fields map[string]any) error {
	return r.db.Model(&model.Emoji{}).Where("id = ?", id).Updates(fields).Error
}

func (r *emojiRepository) FindManyByIDs(ids []string) ([]*model.Emoji, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var emojis []*model.Emoji
	if err := r.db.Where("id IN ?", ids).Find(&emojis).Error; err != nil {
		return nil, err
	}
	return emojis, nil
}

func (r *emojiRepository) UpdateFieldsMany(ids []string, fields map[string]any) error {
	if len(ids) == 0 || len(fields) == 0 {
		return nil
	}
	return r.db.Model(&model.Emoji{}).Where("id IN ?", ids).Updates(fields).Error
}

func (r *emojiRepository) Delete(id string) error {
	return r.db.Where("id = ?", id).Delete(&model.Emoji{}).Error
}

func (r *emojiRepository) DeleteMany(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.Where("id IN ?", ids).Delete(&model.Emoji{}).Error
}

func (r *emojiRepository) ListRemoteWithFilter(query, host string, limit, offset int) ([]*model.Emoji, error) {
	q := r.db.Where("host IS NOT NULL").Order("id DESC")
	if query != "" {
		q = q.Where("name ILIKE ?", "%"+query+"%")
	}
	if host != "" {
		q = q.Where("host = ?", host)
	}
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	q = q.Limit(limit)
	if offset > 0 {
		q = q.Offset(offset)
	}
	var emojis []*model.Emoji
	if err := q.Find(&emojis).Error; err != nil {
		return nil, err
	}
	return emojis, nil
}

func (r *emojiRepository) ListWithFilter(query, category string, local bool, limit, offset int) ([]*model.Emoji, error) {
	q := r.db.Order("id ASC")
	if local {
		q = q.Where("host IS NULL")
	}
	if query != "" {
		q = q.Where("name ILIKE ?", "%"+query+"%")
	}
	if category != "" {
		q = q.Where("category = ?", category)
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	q = q.Limit(limit)
	if offset > 0 {
		q = q.Offset(offset)
	}
	var emojis []*model.Emoji
	if err := q.Find(&emojis).Error; err != nil {
		return nil, err
	}
	return emojis, nil
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
