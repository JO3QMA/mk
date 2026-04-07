package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// PollVoteRepository provides data access for the `poll_vote` table.
type PollVoteRepository interface {
	Create(v *model.PollVote) error
	FindByUserAndChoice(userID, noteID string, choice int) (*model.PollVote, error)
	CountByUserAndNote(userID, noteID string) (int64, error)
	ListByNoteID(noteID string) ([]*model.PollVote, error)
}

type pollVoteRepository struct {
	db *gorm.DB
}

// NewPollVoteRepository creates a new PollVoteRepository.
func NewPollVoteRepository(db *gorm.DB) PollVoteRepository {
	return &pollVoteRepository{db: db}
}

func (r *pollVoteRepository) Create(v *model.PollVote) error {
	return r.db.Create(v).Error
}

// FindByUserAndChoice returns the vote for the given (user, note, choice) triple.
func (r *pollVoteRepository) FindByUserAndChoice(userID, noteID string, choice int) (*model.PollVote, error) {
	var v model.PollVote
	if err := r.db.
		Where("\"userId\" = ? AND \"noteId\" = ? AND choice = ?", userID, noteID, choice).
		First(&v).Error; err != nil {
		return nil, err
	}
	return &v, nil
}

// CountByUserAndNote returns the number of votes the user has cast on the note.
// 単一選択の重複検知に使用する。
func (r *pollVoteRepository) CountByUserAndNote(userID, noteID string) (int64, error) {
	var count int64
	if err := r.db.Model(&model.PollVote{}).
		Where("\"userId\" = ? AND \"noteId\" = ?", userID, noteID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// ListByNoteID returns all votes for a note (used for entity packing).
func (r *pollVoteRepository) ListByNoteID(noteID string) ([]*model.PollVote, error) {
	var rows []*model.PollVote
	if err := r.db.Where("\"noteId\" = ?", noteID).
		Order("id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
