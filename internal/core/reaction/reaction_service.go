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
	// ErrBlocked is returned when the note author has blocked the reactor.
	ErrBlocked = errors.New("blocked by note author")
)

// BlockingChecker reports whether one user has blocked another. パッケージ間の
// 循環依存を避けるためinterfaceで受け取る (実装は core/blocking)。
type BlockingChecker interface {
	IsBlocked(blockerID, blockeeID string) (bool, error)
}

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

// FederationHook is invoked after a reaction is created or removed so that
// the ActivityPub layer can deliver Like / Undo Like activities. パッケージ間
// の循環依存を避けるためinterfaceで受け取る (実装は core/federation)。
type FederationHook interface {
	OnReactionAdded(reactor *model.User, target *model.Note, reaction string)
	OnReactionRemoved(reactor *model.User, target *model.Note, reaction string)
}

// ChartHook is invoked after a reaction is created so the chart
// subsystem can record per-user reaction counts. パッケージ間の循環
// 依存を避けるため interface で受け取る (実装は core/chart/charthook)。
type ChartHook interface {
	OnReactionCreated(reactor *model.User, note *model.Note)
}

// WebhookHook is invoked after a reaction has been created so that user
// webhooks subscribed to the `reaction` event can fire. 循環依存を避けるため
// interface で受け取る (実装は core/webhook)。
type WebhookHook interface {
	OnReactionCreated(note *model.Note, reactor *model.User, reaction string)
}

// Service manages note reactions.
type Service struct {
	noteRepo         repository.NoteRepository
	reactionRepo     repository.NoteReactionRepository
	emojiRepo        repository.EmojiRepository
	followingRepo    repository.FollowingRepository
	idGen            id.Generator
	notificationHook NotificationHook
	blockingChecker  BlockingChecker
	federationHook   FederationHook
	chartHook        ChartHook
	webhookHook      WebhookHook
	countWriter      ReactionCountWriter
}

// SetCountWriter replaces the default direct writer with a buffered one.
func (s *Service) SetCountWriter(w ReactionCountWriter) {
	s.countWriter = w
}

// CountWriter returns the active ReactionCountWriter (for read merge).
func (s *Service) CountWriter() ReactionCountWriter {
	return s.countWriter
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
		countWriter:   NewDirectWriter(noteRepo),
	}
}

// SetNotificationHook attaches a NotificationHook used after Create succeeds.
func (s *Service) SetNotificationHook(h NotificationHook) {
	s.notificationHook = h
}

// SetBlockingChecker attaches a BlockingChecker used by Create.
func (s *Service) SetBlockingChecker(c BlockingChecker) {
	s.blockingChecker = c
}

// SetFederationHook attaches a FederationHook used after Create / Delete
// succeed.
func (s *Service) SetFederationHook(h FederationHook) {
	s.federationHook = h
}

// SetChartHook attaches a ChartHook invoked after a reaction is
// created so the chart subsystem can record the event.
func (s *Service) SetChartHook(h ChartHook) {
	s.chartHook = h
}

// SetWebhookHook attaches a WebhookHook invoked after a reaction has been
// created so that user webhooks subscribed to the reaction event can fire.
func (s *Service) SetWebhookHook(h WebhookHook) {
	s.webhookHook = h
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

	// 投稿者にブロックされている場合はリアクション不可
	if s.blockingChecker != nil && target.UserID != user.ID {
		if blocked, err := s.blockingChecker.IsBlocked(target.UserID, user.ID); err != nil {
			return "", err
		} else if blocked {
			return "", ErrBlocked
		}
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
		_ = s.countWriter.Increment(target.ID, existing.Reaction, -1)
		// 連合先には古いリアクションを Undo Like で送る
		if s.federationHook != nil {
			s.federationHook.OnReactionRemoved(user, target, existing.Reaction)
		}
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
	_ = s.countWriter.Increment(target.ID, reaction, 1)

	// 通知発行 (自分自身へのリアクションは内部で抑制される)
	if s.notificationHook != nil && target.UserID != user.ID {
		s.notificationHook.OnReactionCreated(target.UserID, user.ID, target.ID, reaction)
	}
	// AP配信もベストエフォート。
	if s.federationHook != nil {
		s.federationHook.OnReactionAdded(user, target, reaction)
	}
	// チャート集計もベストエフォート。
	if s.chartHook != nil {
		s.chartHook.OnReactionCreated(user, target)
	}
	// Webhook配信もベストエフォート。
	if s.webhookHook != nil {
		s.webhookHook.OnReactionCreated(target, user, reaction)
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
	_ = s.countWriter.Increment(target.ID, existing.Reaction, -1)
	if s.federationHook != nil {
		s.federationHook.OnReactionRemoved(user, target, existing.Reaction)
	}
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
	var reactions []string
	if reaction != "" {
		reactions = reactionVariants(s.normalizeReaction(reaction))
	}
	return s.reactionRepo.ListByNoteID(target.ID, untilID, sinceID, limit, reactions)
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
		// "@." はローカルを表すcanonical suffix (TS互換)
		if host == "." {
			host = ""
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

// localCanonicalPattern matches the canonical local emoji form `:name@.:`.
var localCanonicalPattern = regexp.MustCompile(`^:([\w+\-]+)@\.:$`)

// reactionVariants returns a slice of reaction strings to match in the DB.
// TS時代のレコードは `:name:` 形式、mk時代は `:name@.:` 形式で保存されて
// いるため、ローカルカスタム絵文字の場合は両方の形式で検索する必要がある。
func reactionVariants(normalized string) []string {
	if m := localCanonicalPattern.FindStringSubmatch(normalized); m != nil {
		return []string{normalized, ":" + m[1] + ":"}
	}
	return []string{normalized}
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
