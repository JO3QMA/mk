package stream

import (
	"context"
	"encoding/json"
	"log/slog"

	coredrive "github.com/shiroha-a/mk/internal/core/drive"
	corenotification "github.com/shiroha-a/mk/internal/core/notification"
	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
)

// PubSubPublisher abstracts the Publish side of core/event.PubSubService.
// 循環依存を避けるため interface で受け取る。
type PubSubPublisher interface {
	Publish(ctx context.Context, channel string, payload any) error
}

// NotePublisher serializes a model.Note via entity.PackNote, embeds the author
// in NoteEntity.User and publishes the JSON to a Redis pubsub topic. これは
// core/timeline.StreamingPublisher を実装する。
type NotePublisher struct {
	pub   PubSubPublisher
	idGen id.Generator
}

// NewNotePublisher constructs a NotePublisher.
func NewNotePublisher(pub PubSubPublisher, idGen id.Generator) *NotePublisher {
	return &NotePublisher{pub: pub, idGen: idGen}
}

// PublishNote implements core/timeline.StreamingPublisher.
func (p *NotePublisher) PublishNote(topic string, n *model.Note, author *model.User) {
	if p.pub == nil || n == nil || author == nil {
		return
	}
	pn := entity.PackNote(n, p.idGen)
	pn.User = entity.PackUserLite(author)
	body, err := json.Marshal(pn)
	if err != nil {
		slog.Warn("note publisher: marshal failed", "err", err)
		return
	}
	if err := p.pub.Publish(context.Background(), topic, json.RawMessage(body)); err != nil {
		slog.Warn("note publisher: publish failed", "topic", topic, "err", err)
	}
}

// NotificationPublisher serializes a notification.Notification and publishes
// it to the per-user `notifications:<id>` topic. Implements
// core/notification.StreamingPublisher.
type NotificationPublisher struct {
	pub PubSubPublisher
}

// NewNotificationPublisher constructs a NotificationPublisher.
func NewNotificationPublisher(pub PubSubPublisher) *NotificationPublisher {
	return &NotificationPublisher{pub: pub}
}

// PublishNotification serializes the notification and publishes to
// notifications:<id>. Marshal 失敗 / publish 失敗は best-effort で握りつぶす。
func (p *NotificationPublisher) PublishNotification(notifieeID string, n *corenotification.Notification) {
	if p.pub == nil || notifieeID == "" || n == nil {
		return
	}
	body, err := json.Marshal(n)
	if err != nil {
		slog.Warn("notification publisher: marshal failed", "err", err)
		return
	}
	topic := "notifications:" + notifieeID
	if err := p.pub.Publish(context.Background(), topic, json.RawMessage(body)); err != nil {
		slog.Warn("notification publisher: publish failed", "topic", topic, "err", err)
	}
}

// DrivePublisher serializes a drive file event and publishes it to the
// per-user `drive:<id>` topic. Implements core/drive.StreamingPublisher.
type DrivePublisher struct {
	pub PubSubPublisher
}

// NewDrivePublisher constructs a DrivePublisher.
func NewDrivePublisher(pub PubSubPublisher) *DrivePublisher {
	return &DrivePublisher{pub: pub}
}

// PublishDriveEvent wraps the drive file in `{type, body}` envelope and
// publishes to drive:<userID>.
func (p *DrivePublisher) PublishDriveEvent(userID, eventType string, file *model.DriveFile) {
	if p.pub == nil || userID == "" || file == nil || eventType == "" {
		return
	}
	envelope := map[string]any{
		"type": eventType,
		"body": file,
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		slog.Warn("drive publisher: marshal failed", "err", err)
		return
	}
	topic := "drive:" + userID
	if err := p.pub.Publish(context.Background(), topic, json.RawMessage(body)); err != nil {
		slog.Warn("drive publisher: publish failed", "topic", topic, "err", err)
	}
}

// 静的アサーション: DrivePublisher が core/drive.StreamingPublisher を満たす。
var _ coredrive.StreamingPublisher = (*DrivePublisher)(nil)
