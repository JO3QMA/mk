// Package reaction provides ReactionService for managing note reactions.
package reaction

import (
	"errors"
	"regexp"
	"time"

	"github.com/shiroha-a/mk/internal/core/note"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

// FallbackReaction is the default reaction used when no reaction is provided.
// 本家Misskeyは Heart "❤" を fallback にしている。
const FallbackReaction = "\u2764"

// Errors returned by Service.
var (
	// ErrNoteNotFound is returned when the target note does not exist.
	ErrNoteNotFound = errors.New("note not found")
	// ErrNoteNotVisible is returned when the user cannot see the target note.
	ErrNoteNotVisible = errors.New("note not visible to user")
	// ErrAlreadyReacted is returned when the user has already reacted with the same reaction.
	ErrAlreadyReacted = errors.New("user has already reacted with this reaction")
	// ErrReactionNotFound is returned when there is no reaction to delete.
	ErrReactionNotFound = errors.New("reaction not found")
	// ErrCannotReactToPureRenote is returned when the user attempts to react to a pure renote.
	ErrCannotReactToPureRenote = errors.New("cannot react to a pure renote")
)

// legacyMap converts legacy text reactions like "like" or "love" to their
// Unicode equivalents. Misskey本家のReactionServiceと同じ表を使用する。
var legacyMap = map[string]string{
	"like":     "👍",
	"love":     "\u2764",
	"laugh":    "😆",
	"hmm":      "🤔",
	"surprise": "😮",
	"congrats": "🎉",
	"angry":    "💢",
	"confused": "😥",
	"rip":      "😇",
	"pudding":  "🍮",
	"star":     "⭐",
}

// customEmojiPattern matches a custom emoji shortcode like ":smile:" or
// ":smile@example.com:".
var customEmojiPattern = regexp.MustCompile(`^:([\w+\-]+)(?:@([\w.\-]+))?:$`)

// NotificationHook is invoked after a reaction is created.
type NotificationHook interface {
	OnReactionCreated(notifieeID, notifierID, noteID, reaction string)
}

// Service manages note reactions.
type Service struct {
	noteRepo         repository.NoteRepository
	reactionRepo     repository.NoteReactionRepository
	emojiRepo        repository.EmojiRepository
	followingRepo    repository.FollowingRepository
	idGen            id.Generator
	notificationHook NotificationHook
}

// NewService constructs a new ReactionService.
func NewService(
	noteRepo repository.NoteRepository,
	reactionRepo repository.NoteReactionRepository,
	emojiRepo repository.EmojiRepository,
	followingRepo repository.FollowingRepository,
	idGen id.Generator,
) *Service {
	return &Service{
		noteRepo:      noteRepo,
		reactionRepo:  reactionRepo,
		emojiRepo:     emojiRepo,
		followingRepo: followingRepo,
		idGen:         idGen,
	}
}

// SetNotificationHook attaches a NotificationHook used after Create succeeds.
func (s *Service) SetNotificationHook(h NotificationHook) {
	s.notificationHook = h
}

// Create attaches a reaction by user to the target note.
// 同じユーザーが既に同じリアクションをしている場合は ErrAlreadyReacted。
// 異なるリアクションを既にしている場合は古い方を削除して新しい方を作成する。
func (s *Service) Create(user *model.User, noteID, rawReaction string) (string, error) {
	if user == nil {
		return "", errors.New("user is required")
	}

	target, err := s.noteRepo.FindByIDWithUser(noteID)
	if err != nil {
		return "", ErrNoteNotFound
	}
	if !note.CanSeeNote(user, target, s.followingRepo) {
		return "", ErrNoteNotVisible
	}

	// pure renote (text/cw/file/poll を伴わない renote) にはリアクションできない
	if isPureRenote(target) {
		return "", ErrCannotReactToPureRenote
	}

	reaction := s.normalizeReaction(rawReaction)

	// 既存リアクションを確認
	if existing, err := s.reactionRepo.FindByPair(user.ID, target.ID); err == nil {
		if existing.Reaction == reaction {
			return "", ErrAlreadyReacted
		}
		// 別のリアクションがすでにあるので置き換える
		if err := s.reactionRepo.Delete(existing); err != nil {
			return "", err
		}
		// 集計列も古いリアクションを-1
		_ = s.noteRepo.IncrementReaction(target.ID, existing.Reaction, -1)
	}

	rec := &model.NoteReaction{
		ID:       s.idGen.Generate(time.Now()),
		UserID:   user.ID,
		NoteID:   target.ID,
		Reaction: reaction,
	}
	if err := s.reactionRepo.Create(rec); err != nil {
		return "", err
	}
	_ = s.noteRepo.IncrementReaction(target.ID, reaction, 1)

	// 通知発行 (自分自身へのリアクションは内部で抑制される)
	if s.notificationHook != nil && target.UserID != user.ID {
		s.notificationHook.OnReactionCreated(target.UserID, user.ID, target.ID, reaction)
	}

	return reaction, nil
}

// Delete removes the user's reaction from the note.
func (s *Service) Delete(user *model.User, noteID string) error {
	if user == nil {
		return errors.New("user is required")
	}
	target, err := s.noteRepo.FindByID(noteID)
	if err != nil {
		return ErrNoteNotFound
	}
	existing, err := s.reactionRepo.FindByPair(user.ID, target.ID)
	if err != nil {
		return ErrReactionNotFound
	}
	if err := s.reactionRepo.Delete(existing); err != nil {
		return err
	}
	_ = s.noteRepo.IncrementReaction(target.ID, existing.Reaction, -1)
	return nil
}

// List returns reactions for the given note id.
func (s *Service) List(user *model.User, noteID, untilID, sinceID string, limit int, reaction string) ([]*model.NoteReaction, error) {
	target, err := s.noteRepo.FindByIDWithUser(noteID)
	if err != nil {
		return nil, ErrNoteNotFound
	}
	if !note.CanSeeNote(user, target, s.followingRepo) {
		return nil, ErrNoteNotVisible
	}
	if limit <= 0 {
		limit = 10
	}
	if reaction != "" {
		reaction = s.normalizeReaction(reaction)
	}
	return s.reactionRepo.ListByNoteID(target.ID, untilID, sinceID, limit, reaction)
}

// normalizeReaction returns the canonical form of a reaction string.
//   - 空の文字列はFallback (heart) に置換
//   - レガシー文字列(like等)はUnicode絵文字に変換
//   - カスタム絵文字 ":name:" は絵文字テーブルで存在確認後 ":name@.:" に正規化
//   - リモート ":name@host:" はそのまま検証して残す
//   - その他はそのまま (Unicode絵文字想定)
func (s *Service) normalizeReaction(raw string) string {
	if raw == "" {
		return FallbackReaction
	}
	if v, ok := legacyMap[raw]; ok {
		return v
	}
	if m := customEmojiPattern.FindStringSubmatch(raw); m != nil {
		name := m[1]
		host := ""
		if len(m) >= 3 {
			host = m[2]
		}
		var hostPtr *string
		if host != "" {
			hostPtr = &host
		}
		if _, err := s.emojiRepo.FindByNameAndHost(name, hostPtr); err == nil {
			if host == "" {
				return ":" + name + "@.:"
			}
			return ":" + name + "@" + host + ":"
		}
		// 見つからなければFallbackにする
		return FallbackReaction
	}
	return raw
}

// isPureRenote reports whether the given note is a pure renote (no text/cw/files/poll).
func isPureRenote(n *model.Note) bool {
	if n.RenoteID == nil {
		return false
	}
	if n.Text != nil && *n.Text != "" {
		return false
	}
	if n.CW != nil && *n.CW != "" {
		return false
	}
	if len(n.FileIDs) > 0 {
		return false
	}
	if n.HasPoll {
		return false
	}
	return true
}
