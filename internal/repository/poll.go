package repository

import (
	"fmt"

	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// PollRepository provides data access for polls.
type PollRepository interface {
	Create(poll *model.Poll) error
	FindByNoteID(noteID string) (*model.Poll, error)
	IncrementVote(noteID string, choice int, delta int) error
}

type pollRepository struct {
	db *gorm.DB
}

// NewPollRepository creates a new PollRepository.
func NewPollRepository(db *gorm.DB) PollRepository {
	return &pollRepository{db: db}
}

func (r *pollRepository) Create(poll *model.Poll) error {
	return r.db.Create(poll).Error
}

// FindByNoteID returns the poll attached to noteID.
func (r *pollRepository) FindByNoteID(noteID string) (*model.Poll, error) {
	var p model.Poll
	if err := r.db.Where("\"noteId\" = ?", noteID).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// IncrementVote bumps poll.votes[choice] by delta. PostgreSQLの配列は1始まりのため
// SQLでは choice+1 を用いる。GORMの UpdateColumn は配列インデックスを取れない
// ため、ここではraw SQLで更新する。choiceは整数なので文字列展開でも安全。
func (r *pollRepository) IncrementVote(noteID string, choice int, delta int) error {
	pgChoice := choice + 1
	q := fmt.Sprintf(`UPDATE "poll" SET votes[%d] = votes[%d] + ? WHERE "noteId" = ?`, pgChoice, pgChoice)
	return r.db.Exec(q, delta, noteID).Error
}
