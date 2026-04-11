// Package note provides core business logic services for notes.
package note

import (
	"errors"
	"regexp"
	"time"

	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

// Errors returned by NoteCreateService.
var (
	// ErrNoteContentRequired is returned when text, fileIds and renoteId are all empty.
	ErrNoteContentRequired = errors.New("text, fileIds, or renoteId is required")
	// ErrReplyTargetNotFound is returned when the replyId references a missing note.
	ErrReplyTargetNotFound = errors.New("reply target not found")
	// ErrRenoteTargetNotFound is returned when the renoteId references a missing note.
	ErrRenoteTargetNotFound = errors.New("renote target not found")
	// ErrCannotReplyToInvisibleNote is returned when the replier cannot see the reply target.
	ErrCannotReplyToInvisibleNote = errors.New("cannot reply to this note")
	// ErrCannotRenoteInvisibleNote is returned when the renoter cannot see the renote target.
	ErrCannotRenoteInvisibleNote = errors.New("cannot renote this note")
	// ErrChannelNotFound is returned when the channelId references a missing channel.
	ErrChannelNotFound = errors.New("channel not found")
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

// TimelineFanoutHook is invoked after a note has been persisted so that the
// timeline subsystem can deliver the note to interested feeds. パッケージ間の
// 循環依存を避けるためinterfaceで受け取る (実装は core/timeline)。
type TimelineFanoutHook interface {
	OnNoteCreated(note *model.Note, author *model.User)
}

// NotificationHook is invoked after a note has been persisted so that the
// notification subsystem can create notification entries for mentions, replies,
// renotes, etc. パッケージ間の循環依存を避けるためinterfaceで受け取る。
type NotificationHook interface {
	OnNoteCreated(note *model.Note, author *model.User, replyTarget, renoteTarget *model.Note)
}

// FederationHook is invoked after a note has been persisted so that the
// ActivityPub layer can deliver the note to remote followers.
// パッケージ間の循環依存を避けるためinterfaceで受け取る (実装は core/federation)。
type FederationHook interface {
	OnNoteCreated(note *model.Note, author *model.User)
}

// ChannelHook is invoked when a note is posted to a channel so the channel
// service can validate existence ahead of insertion and bump counters / the
// last-noted timestamp afterwards. パッケージ間の循環依存を避けるため
// interface で受け取る (実装は core/channel)。
type ChannelHook interface {
	// EnsureChannelExists is called before insertion. Returning a non-nil
	// error aborts the note creation.
	EnsureChannelExists(channelID string) error
	// OnNotePosted is called after the note row has been persisted.
	OnNotePosted(channelID string)
}

// AntennaHook is invoked after a note has been persisted so antenna service
// can fan it out into matching antenna timelines. パッケージ間の循環依存を
// 避けるため interface で受け取る (実装は core/antenna)。
type AntennaHook interface {
	OnNoteCreated(note *model.Note, author *model.User)
}

// IndexHook is invoked after a note has been persisted (or deleted) so the
// search subsystem can update its full-text index. 失敗してもメイン処理に
// 影響させたくないため、戻り値は捨てる前提のベストエフォート呼び出し。
// パッケージ間の循環依存を避けるため interface で受け取る (実装は core/search)。
type IndexHook interface {
	OnNoteCreated(note *model.Note)
	OnNoteDeleted(note *model.Note)
}

// ChartHook is invoked after a note has been persisted so the chart
// subsystem can fan the event into NotesChart, PerUserNotesChart,
// InstanceChart and ActiveUsersChart. パッケージ間の循環依存を避ける
// ため interface で受け取る (実装は core/chart/charthook)。
type ChartHook interface {
	OnNoteCreated(note *model.Note)
}

// CreateService provides note creation logic.
type CreateService struct {
	noteRepo         repository.NoteRepository
	pollRepo         repository.PollRepository
	followingRepo    repository.FollowingRepository
	idGen            id.Generator
	fanoutHook       TimelineFanoutHook
	notificationHook NotificationHook
	federationHook   FederationHook
	channelHook      ChannelHook
	antennaHook      AntennaHook
	indexHook        IndexHook
	chartHook        ChartHook
	userRepo         repository.UserRepository
}

// SetUserRepo attaches a UserRepository for resolving mention usernames to IDs.
func (s *CreateService) SetUserRepo(r repository.UserRepository) {
	s.userRepo = r
}

// NewCreateService creates a new CreateService.
// followingRepo は省略可 (nil)。指定された場合、followers可視性ノートへの
// reply/renoteで閲覧権限を厳格にチェックする。
// fanoutHookも省略可 (nil)。設定されていればnote作成成功時にコールバックされる。
func NewCreateService(
	noteRepo repository.NoteRepository,
	pollRepo repository.PollRepository,
	idGen id.Generator,
	followingRepo repository.FollowingRepository,
) *CreateService {
	return &CreateService{
		noteRepo:      noteRepo,
		pollRepo:      pollRepo,
		followingRepo: followingRepo,
		idGen:         idGen,
	}
}

// SetFanoutHook attaches a TimelineFanoutHook to be invoked on note creation.
// 後付けセッターにすることで、Wireのような循環依存問題を起こさず、テストでも
// 簡単に差し替え可能にする。
func (s *CreateService) SetFanoutHook(h TimelineFanoutHook) {
	s.fanoutHook = h
}

// SetNotificationHook attaches a NotificationHook invoked on note creation.
func (s *CreateService) SetNotificationHook(h NotificationHook) {
	s.notificationHook = h
}

// SetFederationHook attaches a FederationHook invoked on note creation.
func (s *CreateService) SetFederationHook(h FederationHook) {
	s.federationHook = h
}

// SetChannelHook attaches a ChannelHook invoked when a note is posted to a
// channel. nil 渡しは無効化と同義。
func (s *CreateService) SetChannelHook(h ChannelHook) {
	s.channelHook = h
}

// SetAntennaHook attaches an AntennaHook invoked after note creation so the
// antenna service can fan the note out into matching antenna timelines.
func (s *CreateService) SetAntennaHook(h AntennaHook) {
	s.antennaHook = h
}

// SetIndexHook attaches an IndexHook invoked after note creation so the
// search backend can index the new note.
func (s *CreateService) SetIndexHook(h IndexHook) {
	s.indexHook = h
}

// SetChartHook attaches a ChartHook invoked after note creation so the
// chart subsystem can record the event in the relevant time series.
func (s *CreateService) SetChartHook(h ChartHook) {
	s.chartHook = h
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

	// channelId が指定されていれば存在チェック。channelHook 未設定なら
	// channel 機能が無効なものとして扱い、エラーは返さない。
	if in.ChannelID != nil && *in.ChannelID != "" && s.channelHook != nil {
		if err := s.channelHook.EnsureChannelExists(*in.ChannelID); err != nil {
			return nil, ErrChannelNotFound
		}
	}

	// reply/renote先のノートを取得し、閲覧権限を確認する。
	// 取得したノートは、後段のカウンタ更新と非正規化フィールドの埋め込みに使う。
	var replyTarget, renoteTarget *model.Note
	if in.ReplyID != nil {
		t, err := s.noteRepo.FindByIDWithUser(*in.ReplyID)
		if err != nil {
			return nil, ErrReplyTargetNotFound
		}
		if !CanSeeNote(in.User, t, s.followingRepo) {
			return nil, ErrCannotReplyToInvisibleNote
		}
		replyTarget = t
	}
	if in.RenoteID != nil {
		t, err := s.noteRepo.FindByIDWithUser(*in.RenoteID)
		if err != nil {
			return nil, ErrRenoteTargetNotFound
		}
		if !CanSeeNote(in.User, t, s.followingRepo) {
			return nil, ErrCannotRenoteInvisibleNote
		}
		renoteTarget = t
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

	// reply/renote先の非正規化フィールドを埋める
	if replyTarget != nil {
		note.ReplyUserID = &replyTarget.UserID
		note.ReplyUserHost = replyTarget.UserHost
	}
	if renoteTarget != nil {
		note.RenoteUserID = &renoteTarget.UserID
		note.RenoteUserHost = renoteTarget.UserHost
		note.RenoteChannelID = renoteTarget.ChannelID
	}

	if in.VisibleUserIDs != nil {
		note.VisibleUserIDs = in.VisibleUserIDs
	}

	// メンションの抽出。ユーザー名+ホストをユーザーIDに解決する。
	// ローカル (host="") は host IS NULL で、リモート (host!="") は
	// host 列の等価比較で lookup する。未知のユーザーは単にスキップする
	// (remote webfinger 解決は pre-lookup されている前提)。
	// userRepo が未設定のときは後方互換のため username 文字列をそのまま格納する。
	if in.Text != nil {
		if s.userRepo != nil {
			mentions := ExtractMentionStructs(*in.Text)
			if len(mentions) > 0 {
				ids := make([]string, 0, len(mentions))
				for _, m := range mentions {
					var host *string
					if m.Host != "" {
						h := m.Host
						host = &h
					}
					if u, err := s.userRepo.FindByUsernameLower(m.Username, host); err == nil {
						ids = append(ids, u.ID)
					}
				}
				note.Mentions = ids
			}
		} else {
			note.Mentions = ExtractMentions(*in.Text)
		}
	}

	if err := s.noteRepo.Create(note); err != nil {
		return nil, err
	}

	// reply/renote先のカウンタを更新する。
	// quote renoteの場合 (text/cw/poll/fileを伴うrenote) は renoteCount を増やさず、
	// 純粋なrenoteのみ集計する。これはMisskey本家の挙動に揃えている。
	if replyTarget != nil {
		_ = s.noteRepo.IncrementCount(replyTarget.ID, "repliesCount", 1)
	}
	if renoteTarget != nil && isPureRenote(in) {
		_ = s.noteRepo.IncrementCount(renoteTarget.ID, "renoteCount", 1)
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
	finalNote := note
	if loaded, err := s.noteRepo.FindByIDWithUser(noteID); err == nil && loaded != nil {
		finalNote = loaded
	} else {
		finalNote.User = in.User
	}

	// 永続化が成功してからtimelineへ配信する。fanoutが失敗してもnote自体は
	// 既に保存済みなので、ハンドラへはエラーを返さずベストエフォートとする。
	if s.fanoutHook != nil {
		s.fanoutHook.OnNoteCreated(finalNote, in.User)
	}
	// 通知も同様にベストエフォート。
	if s.notificationHook != nil {
		s.notificationHook.OnNoteCreated(finalNote, in.User, replyTarget, renoteTarget)
	}
	// AP配信もベストエフォート。
	if s.federationHook != nil {
		s.federationHook.OnNoteCreated(finalNote, in.User)
	}
	// チャンネル投稿の lastNotedAt / notesCount 更新もベストエフォート。
	if s.channelHook != nil && in.ChannelID != nil && *in.ChannelID != "" {
		s.channelHook.OnNotePosted(*in.ChannelID)
	}
	// アンテナの fan-out もベストエフォート (失敗してもノート作成自体は成功)。
	if s.antennaHook != nil {
		s.antennaHook.OnNoteCreated(finalNote, in.User)
	}
	// 検索インデックスへの反映もベストエフォート。
	if s.indexHook != nil {
		s.indexHook.OnNoteCreated(finalNote)
	}
	// チャート集計もベストエフォート。失敗してもノート作成自体は成功扱い。
	if s.chartHook != nil {
		s.chartHook.OnNoteCreated(finalNote)
	}

	// ユーザーのnotesCountをインクリメント (ベストエフォート)
	_ = s.noteRepo.IncrementUserNotesCount(in.User.ID, 1)

	return finalNote, nil
}

// mentionRegex matches @username and @username@host occurrences anywhere in
// text. Misskey本家のクライアント実装(misskey-js)と同じく、ユーザー名は
// 英数とアンダースコアおよびハイフンからなり、長さは制限しない。
// ホスト部はドメイン形式 (英数・ハイフン・ドット) を許容する。
var mentionRegex = regexp.MustCompile(`@([A-Za-z0-9_-]+)(?:@([A-Za-z0-9.\-]+))?`)

// Mention represents a single user mention extracted from note text.
type Mention struct {
	Username string
	Host     string // ホスト指定がない場合は空文字
}

// ExtractMentions extracts mention usernames from a note text.
// 戻り値はusername (ホストなしの場合) または "username@host" (リモートユーザー指定の場合)。
// Misskeyのnote.mentions列はユーザーIDの配列だが、本サービスではユーザー解決を
// 別レイヤで行う前提で、ここではユーザー名形式のままで返す。重複は除去する。
func ExtractMentions(text string) []string {
	matches := mentionRegex.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	var out []string
	for _, m := range matches {
		username := m[1]
		host := ""
		if len(m) >= 3 {
			host = m[2]
		}
		key := username
		if host != "" {
			key = username + "@" + host
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

// ExtractMentionStructs returns the mentions as structured Mention values.
// 主にNotificationServiceなどリモート/ローカル区別を要する呼び出し向け。
func ExtractMentionStructs(text string) []Mention {
	matches := mentionRegex.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	var out []Mention
	for _, m := range matches {
		username := m[1]
		host := ""
		if len(m) >= 3 {
			host = m[2]
		}
		key := username + "@" + host
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, Mention{Username: username, Host: host})
	}
	return out
}

// isPureRenote reports whether the create request is a pure renote (no text,
// no cw, no poll, no files). 純粋なrenoteのみがrenoteCountに反映される。
func isPureRenote(in CreateInput) bool {
	if in.RenoteID == nil {
		return false
	}
	if in.Text != nil && *in.Text != "" {
		return false
	}
	if in.CW != nil && *in.CW != "" {
		return false
	}
	if len(in.FileIDs) > 0 {
		return false
	}
	if in.Poll != nil && len(in.Poll.Choices) > 0 {
		return false
	}
	return true
}

// IsPureRenote reports whether the persisted note is a pure renote (no text,
// no cw, no poll, no files). 連合先で Create と Announce を切り替える際に使う。
func IsPureRenote(n *model.Note) bool {
	if n == nil || n.RenoteID == nil {
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
