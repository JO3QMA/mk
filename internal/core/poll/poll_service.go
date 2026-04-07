// Package poll provides PollService for casting votes on note polls.
package poll

import (
	"errors"
	"time"

	"github.com/shiroha-a/mk/internal/core/note"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

// Errors returned by Service.
var (
	// ErrNoteNotFound is returned when the target note does not exist.
	ErrNoteNotFound = errors.New("note not found")
	// ErrNoteNotVisible is returned when the user cannot see the target note.
	ErrNoteNotVisible = errors.New("note not visible to user")
	// ErrNoPoll is returned when the target note has no poll attached.
	ErrNoPoll = errors.New("note has no poll")
	// ErrInvalidChoice is returned when the choice index is out of range.
	ErrInvalidChoice = errors.New("invalid choice")
	// ErrPollExpired is returned when the poll has already expired.
	ErrPollExpired = errors.New("poll has expired")
	// ErrAlreadyVoted is returned when the user has already voted.
	ErrAlreadyVoted = errors.New("already voted")
)

// NotificationHook is invoked after a vote is recorded so the notification
// subsystem can deliver a "pollVote" notification to the note author.
type NotificationHook interface {
	OnPollVote(notifieeID, notifierID, noteID string, choice int)
}

// Service manages poll voting.
type Service struct {
	noteRepo         repository.NoteRepository
	pollRepo         repository.PollRepository
	pollVoteRepo     repository.PollVoteRepository
	followingRepo    repository.FollowingRepository
	idGen            id.Generator
	notificationHook NotificationHook
	nowFn            func() time.Time
}

// NewService constructs a PollService.
// followingRepo は followers 可視性ノートへの投票チェックに使われる (省略可)。
func NewService(
	noteRepo repository.NoteRepository,
	pollRepo repository.PollRepository,
	pollVoteRepo repository.PollVoteRepository,
	followingRepo repository.FollowingRepository,
	idGen id.Generator,
) *Service {
	return &Service{
		noteRepo:      noteRepo,
		pollRepo:      pollRepo,
		pollVoteRepo:  pollVoteRepo,
		followingRepo: followingRepo,
		idGen:         idGen,
		nowFn:         time.Now,
	}
}

// SetNotificationHook attaches a NotificationHook used after a successful vote.
func (s *Service) SetNotificationHook(h NotificationHook) {
	s.notificationHook = h
}

// Vote records a vote for the given user/note/choice.
func (s *Service) Vote(user *model.User, noteID string, choice int) error {
	if user == nil {
		return errors.New("user is required")
	}

	target, err := s.noteRepo.FindByIDWithUser(noteID)
	if err != nil {
		return ErrNoteNotFound
	}
	if !note.CanSeeNote(user, target, s.followingRepo) {
		return ErrNoteNotVisible
	}
	if !target.HasPoll {
		return ErrNoPoll
	}

	p, err := s.pollRepo.FindByNoteID(target.ID)
	if err != nil {
		return ErrNoPoll
	}

	if choice < 0 || choice >= len(p.Choices) {
		return ErrInvalidChoice
	}
	if p.ExpiresAt != nil && !s.nowFn().Before(*p.ExpiresAt) {
		return ErrPollExpired
	}

	// 既に同じ choice に投票していたら重複エラー
	if _, err := s.pollVoteRepo.FindByUserAndChoice(user.ID, target.ID, choice); err == nil {
		return ErrAlreadyVoted
	}
	// single モードで既に何かに投票していたらエラー
	if !p.Multiple {
		count, err := s.pollVoteRepo.CountByUserAndNote(user.ID, target.ID)
		if err != nil {
			return err
		}
		if count > 0 {
			return ErrAlreadyVoted
		}
	}

	v := &model.PollVote{
		ID:        s.idGen.Generate(s.nowFn()),
		UserID:    user.ID,
		NoteID:    target.ID,
		Choice:    choice,
		CreatedAt: s.nowFn(),
	}
	if err := s.pollVoteRepo.Create(v); err != nil {
		return err
	}
	_ = s.pollRepo.IncrementVote(target.ID, choice, 1)

	// 投票通知 (自分のノートへの投票はスキップ)
	if s.notificationHook != nil && target.UserID != user.ID {
		s.notificationHook.OnPollVote(target.UserID, user.ID, target.ID, choice)
	}

	return nil
}
