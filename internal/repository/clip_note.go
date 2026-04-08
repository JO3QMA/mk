package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// ClipNoteRepository provides data access for the `clip_note` table.
type ClipNoteRepository interface {
	Create(cn *model.ClipNote) error
	Delete(cn *model.ClipNote) error
	FindByPair(clipID, noteID string) (*model.ClipNote, error)
	ListByClip(clipID string, untilID, sinceID string, limit int) ([]*model.ClipNote, error)
}

type clipNoteRepository struct {
	db *gorm.DB
}

// NewClipNoteRepository creates a new ClipNoteRepository.
func NewClipNoteRepository(db *gorm.DB) ClipNoteRepository {
	return &clipNoteRepository{db: db}
}

func (r *clipNoteRepository) Create(cn *model.ClipNote) error {
	return r.db.Create(cn).Error
}

func (r *clipNoteRepository) Delete(cn *model.ClipNote) error {
	return r.db.Delete(cn).Error
}

func (r *clipNoteRepository) FindByPair(clipID, noteID string) (*model.ClipNote, error) {
	var cn model.ClipNote
	if err := r.db.Where("\"clipId\" = ? AND \"noteId\" = ?", clipID, noteID).First(&cn).Error; err != nil {
		return nil, err
	}
	return &cn, nil
}

// ListByClip returns the entries for a clip ordered by id desc, with
// since/until pagination on the clip_note id.
func (r *clipNoteRepository) ListByClip(clipID string, untilID, sinceID string, limit int) ([]*model.ClipNote, error) {
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	q := r.db.Where("\"clipId\" = ?", clipID)
	if untilID != "" {
		q = q.Where("id < ?", untilID)
	}
	if sinceID != "" {
		q = q.Where("id > ?", sinceID)
	}
	var rows []*model.ClipNote
	if err := q.Order("id DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
