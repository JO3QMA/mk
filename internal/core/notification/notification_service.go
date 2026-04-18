// Package notification provides NotificationService for delivering and reading
// per-user notifications backed by Redis Streams.
package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shiroha-a/mk/internal/misc/id"
)

// Type enumerates the supported notification types.
// Misskey本家と互換のキー名を使用する。
type Type string

const (
	TypeFollow              Type = "follow"
	TypeMention             Type = "mention"
	TypeReply               Type = "reply"
	TypeRenote              Type = "renote"
	TypeQuote               Type = "quote"
	TypeReaction            Type = "reaction"
	TypePollVote            Type = "pollVote"
	TypeReceiveFollowReq    Type = "receiveFollowRequest"
	TypeFollowRequestAccept Type = "followRequestAccepted"
	TypeExportCompleted     Type = "exportCompleted"
	TypeImportCompleted     Type = "importCompleted"
)

// MaxPerUser caps how many notifications are kept per user in the Redis stream.
// 上限に達したら古いものから削除される (XADD MAXLEN ~)。
const MaxPerUser = 300

// streamKey returns the Redis Stream key for a user's notification timeline.
func streamKey(userID string) string {
	return "notificationTimeline:" + userID
}

// readKey returns the Redis key that stores the latest read notification id.
func readKey(userID string) string {
	return "latestReadNotification:" + userID
}

// Notification is the payload stored in Redis.
type Notification struct {
	ID         string         `json:"id"`
	CreatedAt  time.Time      `json:"createdAt"`
	Type       Type           `json:"type"`
	NotifierID string         `json:"notifierId,omitempty"`
	NoteID     string         `json:"noteId,omitempty"`
	Reaction   string         `json:"reaction,omitempty"`
	Choice     *int           `json:"choice,omitempty"`
	Extra      map[string]any `json:"extra,omitempty"`
}

// CreateInput is the parameter set for Service.Create.
type CreateInput struct {
	NotifieeID string
	NotifierID string
	Type       Type
	NoteID     string
	Reaction   string
	Choice     *int
	Extra      map[string]any
}

// StreamingPublisher receives a freshly created notification so that
// WebSocket subscribers can be pushed immediately. パッケージ間の循環依存を
// 避けるため interface で受け取る (実装は internal/stream)。
type StreamingPublisher interface {
	PublishNotification(notifieeID string, n *Notification)
}

// Service manages notifications.
type Service struct {
	client    *redis.Client
	idGen     id.Generator
	publisher StreamingPublisher
}

// NewService constructs a new NotificationService.
func NewService(client *redis.Client, idGen id.Generator) *Service {
	return &Service{client: client, idGen: idGen}
}

// SetStreamingPublisher attaches a StreamingPublisher invoked best-effort
// after Create persists a notification.
func (s *Service) SetStreamingPublisher(p StreamingPublisher) {
	s.publisher = p
}

// Errors returned by Service.
var (
	// ErrSelfNotification is returned when attempting to create a notification where notifier == notifiee.
	ErrSelfNotification = errors.New("cannot notify oneself")
)

// Create writes a notification entry to the user's notification stream.
// notifier == notifiee の場合は何もしない (Misskey本家の挙動を踏襲)。
func (s *Service) Create(ctx context.Context, in CreateInput) (*Notification, error) {
	if in.NotifieeID == "" {
		return nil, errors.New("notifieeId is required")
	}
	if in.NotifierID != "" && in.NotifierID == in.NotifieeID {
		return nil, ErrSelfNotification
	}

	now := time.Now()
	n := &Notification{
		ID:         s.idGen.Generate(now),
		CreatedAt:  now,
		Type:       in.Type,
		NotifierID: in.NotifierID,
		NoteID:     in.NoteID,
		Reaction:   in.Reaction,
		Choice:     in.Choice,
		Extra:      in.Extra,
	}

	payload, err := json.Marshal(n)
	if err != nil {
		return nil, fmt.Errorf("notification marshal: %w", err)
	}
	// MAXLEN ~ で古い通知を確率的にtrim、IDフィールドはGorm IDを利用する
	streamID := toXAddID(n.ID, now)
	if err := s.client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey(in.NotifieeID),
		MaxLen: MaxPerUser,
		Approx: true,
		ID:     streamID,
		Values: map[string]any{"data": string(payload)},
	}).Err(); err != nil {
		return nil, fmt.Errorf("notification xadd: %w", err)
	}
	if s.publisher != nil {
		s.publisher.PublishNotification(in.NotifieeID, n)
	}
	return n, nil
}

// List returns the notifications for the given user, newest first, capped to limit.
func (s *Service) List(ctx context.Context, userID string, limit int) ([]*Notification, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	res, err := s.client.XRevRangeN(ctx, streamKey(userID), "+", "-", int64(limit)).Result()
	if err != nil {
		return nil, err
	}
	out := make([]*Notification, 0, len(res))
	for _, msg := range res {
		raw, ok := msg.Values["data"].(string)
		if !ok {
			continue
		}
		var n Notification
		if err := json.Unmarshal([]byte(raw), &n); err != nil {
			continue
		}
		out = append(out, &n)
	}
	return out, nil
}

// MarkAllAsRead records the latest notification id as read for the user.
// 既読位置を更新するだけで、ストリーム自体は変更しない。
func (s *Service) MarkAllAsRead(ctx context.Context, userID string) error {
	res, err := s.client.XRevRangeN(ctx, streamKey(userID), "+", "-", 1).Result()
	if err != nil {
		return err
	}
	if len(res) == 0 {
		return nil
	}
	return s.client.Set(ctx, readKey(userID), res[0].ID, 0).Err()
}

// LatestReadID returns the most recently read notification id for the user.
// 未読履歴がなければ空文字を返す。
func (s *Service) LatestReadID(ctx context.Context, userID string) (string, error) {
	val, err := s.client.Get(ctx, readKey(userID)).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return val, nil
}

// UnreadCount returns the number of notifications in the user's stream that
// are newer than the stored latest-read id. When there is no read marker (i.e.
// the user has never marked anything as read) the full stream length is
// returned. Callers typically derive hasUnreadNotification = UnreadCount > 0.
func (s *Service) UnreadCount(ctx context.Context, userID string) (int64, error) {
	readID, err := s.LatestReadID(ctx, userID)
	if err != nil {
		return 0, err
	}
	// Redis Streams は ID 単調増加。XLen で全件数を取り、readID 以前の
	// 件数を差し引く方が O(1) でシンプルだが、readID より古いエントリが
	// MAXLEN で削除されている場合があるので XRevRange で直接数える。
	res, err := s.client.XRevRangeN(ctx, streamKey(userID), "+", exclusive(readID), MaxPerUser).Result()
	if err != nil {
		return 0, err
	}
	return int64(len(res)), nil
}

// HasUnreadOfTypes reports whether the user has any unread notification whose
// type is in the provided list. Used by /api/i to compute hasUnreadMentions
// (types: mention/reply). Returns false when types is empty.
// readID 以降のエントリを XRevRange で読み、一件でもマッチしたら true。
func (s *Service) HasUnreadOfTypes(ctx context.Context, userID string, types []Type) (bool, error) {
	if len(types) == 0 {
		return false, nil
	}
	readID, err := s.LatestReadID(ctx, userID)
	if err != nil {
		return false, err
	}
	res, err := s.client.XRevRangeN(ctx, streamKey(userID), "+", exclusive(readID), MaxPerUser).Result()
	if err != nil {
		return false, err
	}
	wanted := make(map[Type]struct{}, len(types))
	for _, t := range types {
		wanted[t] = struct{}{}
	}
	for _, msg := range res {
		raw, ok := msg.Values["data"].(string)
		if !ok {
			continue
		}
		var n Notification
		if err := json.Unmarshal([]byte(raw), &n); err != nil {
			continue
		}
		if _, ok := wanted[n.Type]; ok {
			return true, nil
		}
	}
	return false, nil
}

// exclusive は XRevRange 用の「この ID より後 (newer)」を表す境界文字列を返す。
// 空文字の場合は "-" で stream 先頭を表す。非空なら "(<id>" で exclusive 指定。
func exclusive(readID string) string {
	if readID == "" {
		return "-"
	}
	return "(" + readID
}

// Flush deletes all notifications and the read marker for a user (used in tests
// and account deletion).
func (s *Service) Flush(ctx context.Context, userID string) error {
	if err := s.client.Del(ctx, streamKey(userID)).Err(); err != nil {
		return err
	}
	return s.client.Del(ctx, readKey(userID)).Err()
}

// toXAddID returns a Redis stream id derived from the notification id.
// Redis Streamsは "ms-seq" 形式を期待する。idGenが生成するIDからのタイム
// スタンプ部分を使い、シーケンスは0固定で十分(MAXLENで管理されるため
// 競合はリトライ不要)。
func toXAddID(_ string, t time.Time) string {
	return fmt.Sprintf("%d-*", t.UnixMilli())
}
