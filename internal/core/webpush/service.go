package webpush

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/shiroha-a/mk/internal/queue"
)

// Service builds Web Push delivery jobs and enqueues them.
// Delivery itself is performed asynchronously by the queue worker; callers
// only observe the enqueue latency.
//
// 本家 PushNotificationService.ts と異なり非同期キュー経由にすることで、
// 通知作成フロー (i.e. note create / follow 等) の主経路を詰まらせないように
// している。
type Service struct {
	enqueuer queue.Enqueuer
}

// NewService returns a new webpush.Service bound to the queue enqueuer.
func NewService(enqueuer queue.Enqueuer) *Service {
	return &Service{enqueuer: enqueuer}
}

// PushNotification enqueues a `notification` push to userID.
// body はnotificationエンティティ相当のmapを想定する。
// 本家の PushNotificationService.pushNotification('notification', packed) に対応する。
func (s *Service) PushNotification(userID string, body map[string]any) {
	s.push(userID, TypeNotification, body)
}

// PushUnreadAntennaNote enqueues an `unreadAntennaNote` push.
// body は { antenna: {id, name}, note: PackedNote } 相当のmap。
func (s *Service) PushUnreadAntennaNote(userID string, body map[string]any) {
	s.push(userID, TypeUnreadAntennaNote, body)
}

// PushReadAllNotifications enqueues a `readAllNotifications` push.
// フロントで通知トーストを閉じる用途。body は空 (undefined) でよい。
func (s *Service) PushReadAllNotifications(userID string) {
	s.push(userID, TypeReadAllNotifications, nil)
}

// PushNewChatMessage enqueues a `newChatMessage` push.
// body は packed ChatMessage。
func (s *Service) PushNewChatMessage(userID string, body map[string]any) {
	s.push(userID, TypeNewChatMessage, body)
}

func (s *Service) push(userID, pushType string, body map[string]any) {
	if s == nil || s.enqueuer == nil || userID == "" {
		return
	}
	truncated := TruncateBody(pushType, body)
	var raw json.RawMessage
	if truncated != nil {
		b, err := json.Marshal(truncated)
		if err != nil {
			slog.Warn("webpush: marshal body failed", "type", pushType, "err", err)
			return
		}
		raw = b
	}
	payload := queue.WebPushPayload{UserID: userID, Type: pushType, Body: raw}
	if err := s.enqueuer.EnqueueWebPush(context.Background(), payload); err != nil {
		slog.Warn("webpush: enqueue failed", "type", pushType, "user", userID, "err", err)
	}
}
