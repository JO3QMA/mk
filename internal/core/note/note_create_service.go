// Package note provides core business logic services for notes.
package note

import (
	"errors"
	"strings"
	"time"

	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

// Errors returned by NoteCreateService.
var (
	// ErrNoteContentRequired is returned when text, fileIds and renoteId are all empty.
	ErrNoteContentRequired = errors.New("text, fileIds, or renoteId is required")
)

// CreateInput is the input parameter for CreateService.Create.
type CreateInput struct {
	User               *model.User
	Text               *string
	CW                 *string
	Visibility         model.NoteVisibility
	VisibleUserIDs     []string
	LocalOnly          bool
	ReactionAcceptance *string
	FileIDs            []string
	ReplyID            *string
	RenoteID           *string
	ChannelID          *string
	Poll               *PollInput
}

// PollInput represents the poll part of a create note input.
type PollInput struct {
	Choices   []string
	Multiple  bool
	ExpiresAt *time.Time
}

// CreateService provides note creation logic.
type CreateService struct {
	noteRepo repository.NoteRepository
	pollRepo repository.PollRepository
	idGen    id.Generator
}

// NewCreateService creates a new CreateService.
func NewCreateService(noteRepo repository.NoteRepository, pollRepo repository.PollRepository, idGen id.Generator) *CreateService {
	return &CreateService{
		noteRepo: noteRepo,
		pollRepo: pollRepo,
		idGen:    idGen,
	}
}

// Create creates a new note. It returns the persisted note (with the User
// relation preloaded when possible).
func (s *CreateService) Create(in CreateInput) (*model.Note, error) {
	if in.User == nil {
		return nil, errors.New("user is required")
	}

	// notes/createのバリデーション: text/fileIds/renoteIdのいずれかが必須
	if (in.Text == nil || *in.Text == "") && in.RenoteID == nil && len(in.FileIDs) == 0 {
		return nil, ErrNoteContentRequired
	}

	visibility := in.Visibility
	if visibility == "" {
		visibility = model.NoteVisibilityPublic
	}

	now := time.Now()
	noteID := s.idGen.Generate(now)

	note := &model.Note{
		ID:                 noteID,
		UserID:             in.User.ID,
		Text:               in.Text,
		CW:                 in.CW,
		Visibility:         visibility,
		LocalOnly:          in.LocalOnly,
		ReactionAcceptance: in.ReactionAcceptance,
		ReplyID:            in.ReplyID,
		RenoteID:           in.RenoteID,
		ChannelID:          in.ChannelID,
		FileIDs:            in.FileIDs,
		UserHost:           in.User.Host,
	}

	if in.VisibleUserIDs != nil {
		note.VisibleUserIDs = in.VisibleUserIDs
	}

	// 簡易的なメンション抽出: Phase 2 Step DでMFM対応の本実装に置き換える
	if in.Text != nil {
		note.Mentions = ExtractMentions(*in.Text)
	}

	if err := s.noteRepo.Create(note); err != nil {
		return nil, err
	}

	// 投票が指定されていればPollレコードを作成しnote.hasPollを更新
	if in.Poll != nil && len(in.Poll.Choices) > 0 {
		votes := make([]int64, len(in.Poll.Choices))
		poll := &model.Poll{
			NoteID:         noteID,
			Multiple:       in.Poll.Multiple,
			Choices:        in.Poll.Choices,
			Votes:          votes,
			NoteVisibility: visibility,
			UserID:         in.User.ID,
			UserHost:       in.User.Host,
			ChannelID:      in.ChannelID,
			ExpiresAt:      in.Poll.ExpiresAt,
		}
		if err := s.pollRepo.Create(poll); err != nil {
			return nil, err
		}
		if err := s.noteRepo.Update(note, "hasPoll", true); err != nil {
			return nil, err
		}
		note.HasPoll = true
	}

	// Userリレーションをpreloadして返す。失敗時は引数のUserをそのまま埋めて返す
	if loaded, err := s.noteRepo.FindByIDWithUser(noteID); err == nil && loaded != nil {
		return loaded, nil
	}
	note.User = in.User
	return note, nil
}

// ExtractMentions extracts mention usernames from a note text.
// This is a temporary simplistic implementation; Phase 2 Step D will replace
// it with a full MFM-aware extractor.
func ExtractMentions(text string) []string {
	var mentions []string
	for w := range strings.FieldsSeq(text) {
		if strings.HasPrefix(w, "@") && len(w) > 1 {
			mentions = append(mentions, strings.TrimPrefix(w, "@"))
		}
	}
	return mentions
}
