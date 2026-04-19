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

// NotificationUserRepo abstracts the FindByID call needed to pack a
// Notification's notifier. Narrow interface so we don't pull the whole
// repository package.
type NotificationUserRepo interface {
	FindByID(id string) (*model.User, error)
}

// NotificationNoteRepo abstracts the FindByIDWithUser call needed to pack
// the referenced note (for reply/mention/reaction/renote/quote/pollEnded).
type NotificationNoteRepo interface {
	FindByIDWithUser(id string) (*model.Note, error)
}

// NotificationPublisher serializes a notification.Notification and publishes
// it to the per-user `notifications:<id>` topic. Implements
// core/notification.StreamingPublisher. When userRepo / noteRepo are set
// the outbound JSON is packed via entity.PackNotification so the WebSocket
// body matches Misskey's NotificationEntityService.pack shape.
type NotificationPublisher struct {
	pub      PubSubPublisher
	userRepo NotificationUserRepo
	noteRepo NotificationNoteRepo
	idGen    id.Generator
}

// NewNotificationPublisher constructs a NotificationPublisher.
func NewNotificationPublisher(pub PubSubPublisher) *NotificationPublisher {
	return &NotificationPublisher{pub: pub}
}

// SetRepos wires the narrow repositories and id generator used by
// PublishNotification to pack user/note references. When unset, publish
// falls back to the raw Notification JSON.
func (p *NotificationPublisher) SetRepos(userRepo NotificationUserRepo, noteRepo NotificationNoteRepo, idGen id.Generator) {
	p.userRepo = userRepo
	p.noteRepo = noteRepo
	p.idGen = idGen
}

// Pack implements core/notification.Packer. Returns the packed map shape
// when repos are wired, otherwise the raw Notification so callers can
// fall back to prior behaviour.
func (p *NotificationPublisher) Pack(n *corenotification.Notification) any {
	if n == nil {
		return nil
	}
	if p.userRepo == nil || p.idGen == nil {
		return n
	}
	var user *model.User
	if n.NotifierID != "" {
		if u, err := p.userRepo.FindByID(n.NotifierID); err == nil {
			user = u
		}
	}
	var note *model.Note
	if n.NoteID != "" && p.noteRepo != nil {
		if n2, err := p.noteRepo.FindByIDWithUser(n.NoteID); err == nil {
			note = n2
		}
	}
	return entity.PackNotification(n, user, note, p.idGen)
}

// PublishNotification serializes the notification and publishes to
// notifications:<id>. Marshal 失敗 / publish 失敗は best-effort で握りつぶす。
// Pack() 経由で TS 互換 shape (repo配線時) またはraw Notification(未配線時)
// に変換して送る。
func (p *NotificationPublisher) PublishNotification(notifieeID string, n *corenotification.Notification) {
	if p.pub == nil || notifieeID == "" || n == nil {
		return
	}
	p.publishPacked(notifieeID, p.Pack(n))
}

// PublishPackedNotification publishes an already-packed body to
// notifications:<id>. Allows callers (e.g. core/notification.Service)
// that invoke Pack once and reuse the result across both the
// notifications stream and the main stream to avoid duplicate DB fetches.
func (p *NotificationPublisher) PublishPackedNotification(notifieeID string, packed any) {
	if p.pub == nil || notifieeID == "" || packed == nil {
		return
	}
	p.publishPacked(notifieeID, packed)
}

// publishPacked marshals payload and publishes to notifications:<id>.
func (p *NotificationPublisher) publishPacked(notifieeID string, payload any) {
	body, err := json.Marshal(payload)
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
