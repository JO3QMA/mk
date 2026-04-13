package repository

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// ErrDuplicateReaction is returned by NoteReactionRepository.Create when the
// (userId, noteId) unique constraint is violated.
var ErrDuplicateReaction = errors.New("user has already reacted to this note")

// NoteReactionRepository provides data access for note_reaction rows.
type NoteReactionRepository interface {
	Create(r *model.NoteReaction) error
	Delete(r *model.NoteReaction) error
	FindByPair(userID, noteID string) (*model.NoteReaction, error)
	// FindByUserAndNoteIDs returns reactions for a user across multiple notes.
	// noteIDをキーとしたmapを返す。
	FindByUserAndNoteIDs(userID string, noteIDs []string) (map[string]*model.NoteReaction, error)
	ListByNoteID(noteID string, untilID, sinceID string, limit int, reaction string) ([]*model.NoteReaction, error)
}

type noteReactionRepository struct {
	db *gorm.DB
}

// NewNoteReactionRepository creates a new NoteReactionRepository.
func NewNoteReactionRepository(db *gorm.DB) NoteReactionRepository {
	return &noteReactionRepository{db: db}
}

func (r *noteReactionRepository) Create(rec *model.NoteReaction) error {
	if err := r.db.Create(rec).Error; err != nil {
		// PostgreSQLの一意制約違反 (23505) を専用エラーに変換する
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicateReaction
		}
		return err
	}
	return nil
}

func (r *noteReactionRepository) Delete(rec *model.NoteReaction) error {
	return r.db.Delete(rec).Error
}

func (r *noteReactionRepository) FindByPair(userID, noteID string) (*model.NoteReaction, error) {
	var rec model.NoteReaction
	if err := r.db.Where("\"userId\" = ? AND \"noteId\" = ?", userID, noteID).First(&rec).Error; err != nil {
		return nil, err
	}
	return &rec, nil
}

func (r *noteReactionRepository) FindByUserAndNoteIDs(userID string, noteIDs []string) (map[string]*model.NoteReaction, error) {
	if len(noteIDs) == 0 {
		return map[string]*model.NoteReaction{}, nil
	}
	var rows []*model.NoteReaction
	if err := r.db.Where("\"userId\" = ? AND \"noteId\" IN ?", userID, noteIDs).Find(&rows).Error; err != nil {
		return nil, err
	}
	m := make(map[string]*model.NoteReaction, len(rows))
	for _, row := range rows {
		m[row.NoteID] = row
	}
	return m, nil
}

// ListByNoteID returns reactions for the given noteID, optionally filtered by
// reaction string. Ordered by id DESC for keyset pagination.
func (r *noteReactionRepository) ListByNoteID(noteID string, untilID, sinceID string, limit int, reaction string) ([]*model.NoteReaction, error) {
	var rows []*model.NoteReaction
	q := r.db.Preload("User").Where("\"noteId\" = ?", noteID)
	if reaction != "" {
		q = q.Where("reaction = ?", reaction)
	}
	if untilID != "" {
		q = q.Where("id < ?", untilID)
	}
	if sinceID != "" {
		q = q.Where("id > ?", sinceID)
	}
	if err := q.Order("id DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
