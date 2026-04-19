// Package chat provides a thin service layer over ChatRepository that
// dispatches real-time delivery events to the matching WebSocket channel.
//
// Phase 9.8 — メッセージ送信 / 削除 / 編集 / 既読マークの各 API はこの service
// を経由することで、(a) DB への永続化と (b) Redis pubsub 経由の WebSocket
// 配信を同時に行う。本家 Misskey の ChatService.createMessageToUser /
// createMessageToRoom / deleteMessage の挙動を移植している。
package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/shiroha-a/mk/internal/activitypub"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

// Errors returned by Service.
var (
	// ErrNotFound is returned when a referenced message or room does not exist.
	ErrNotFound = errors.New("chat resource not found")
	// ErrForbidden is returned when the acting user cannot modify the target
	// resource (e.g. deleting someone else's message, posting to a room they
	// are not a member of).
	ErrForbidden = errors.New("chat action forbidden")
	// ErrInvalidTarget is returned when neither toUserId nor toRoomId is
	// provided to CreateMessage helpers.
	ErrInvalidTarget = errors.New("chat target is required")
)

// StreamingPublisher is the Redis pub/sub dispatch interface the service uses
// to broadcast chat events. Implementations live in internal/stream so the
// concrete pub/sub bus can be injected without creating a cycle.
//
// 本家 Misskey の GlobalEventService.publishChatUserStream /
// publishChatRoomStream に対応する。
type StreamingPublisher interface {
	PublishUserMessage(ctx context.Context, fromUserID, toUserID, eventType string, body any)
	PublishRoomMessage(ctx context.Context, roomID, eventType string, body any)
}

// APDeliverer delivers a pre-rendered activity body to a single remote user's
// inbox. Implemented by federation.DeliverService.DeliverToUser.
type APDeliverer interface {
	DeliverToUser(signerUserID string, recipient *model.User, body []byte) error
}

// MainStreamPublisher emits real-time events to a single target user's `main`
// WebSocket channel. Used here to deliver `newChatMessage` events to message
// recipients so the frontend can surface unread-chat indicators immediately.
// 循環依存を避けるため interface で受け取る (実装は internal/stream)。
type MainStreamPublisher interface {
	PublishMainEvent(userID, eventType string, body any)
}

// Event types mirrored by the WebSocket channel. 本家と同名。
const (
	EventMessage = "message"
	EventDeleted = "deleted"
	EventEdited  = "edited"
	EventRead    = "read"
)

// Service implements chat message CRUD + streaming fan-out.
type Service struct {
	repo                repository.ChatRepository
	idGen               id.Generator
	publisher           StreamingPublisher
	userRepo            repository.UserRepository
	renderer            *activitypub.Renderer
	urls                *activitypub.URLBuilder
	deliverer           APDeliverer
	mainStreamPublisher MainStreamPublisher
}

// NewService constructs a chat Service.
func NewService(repo repository.ChatRepository, idGen id.Generator) *Service {
	return &Service{repo: repo, idGen: idGen}
}

// SetStreamingPublisher wires a StreamingPublisher so state-changing methods
// can dispatch to the WebSocket channel. nil 渡しで配信は無効になる (既存の
// DB 処理は変わらない)。
func (s *Service) SetStreamingPublisher(p StreamingPublisher) {
	s.publisher = p
}

// SetMainStreamPublisher attaches a publisher used to emit `newChatMessage`
// on the recipient(s)' main channel. Optional — nil disables main-stream emit
// but chat-stream `message` events remain unaffected.
func (s *Service) SetMainStreamPublisher(p MainStreamPublisher) {
	s.mainStreamPublisher = p
}

// SetAPDelivery wires the AP federation layer for chat message delivery to
// remote users. All parameters are required; pass nil to disable federation.
func (s *Service) SetAPDelivery(userRepo repository.UserRepository, renderer *activitypub.Renderer, urls *activitypub.URLBuilder, deliverer APDeliverer) {
	s.userRepo = userRepo
	s.renderer = renderer
	s.urls = urls
	s.deliverer = deliverer
}

// --- Message lifecycle ---

// CreateMessageToUser persists a direct-message row and fires the
// `message` event on both conversation directions so that either user's
// connected WebSocket subscribers see the message immediately.
func (s *Service) CreateMessageToUser(ctx context.Context, fromUserID, toUserID, text, fileID string) (*model.ChatMessage, error) {
	if fromUserID == "" || toUserID == "" {
		return nil, ErrInvalidTarget
	}
	msg := &model.ChatMessage{
		ID:         s.idGen.Generate(time.Now()),
		FromUserID: fromUserID,
		ToUserID:   &toUserID,
	}
	if text != "" {
		msg.Text = &text
	}
	if fileID != "" {
		msg.FileID = &fileID
	}
	if err := s.repo.CreateMessage(msg); err != nil {
		return nil, fmt.Errorf("create user message: %w", err)
	}
	if s.publisher != nil {
		s.publisher.PublishUserMessage(ctx, fromUserID, toUserID, EventMessage, packMessage(msg))
	}
	// Misskey 本家の ChatService.createMessageToUser は、3 秒後に未読判定して
	// newChatMessage を publishMainStream する。本実装では未読判定を省略し、
	// 作成と同時に recipient (toUserID) の main に emit する (sender には
	// 送らない: TS と同じ fan-out 方針)。
	if s.mainStreamPublisher != nil {
		s.mainStreamPublisher.PublishMainEvent(toUserID, "newChatMessage", packMessage(msg))
	}
	// リモートユーザー宛ならAP配送 (best-effort)
	s.tryDeliverToRemoteUser(msg, fromUserID, toUserID)
	return msg, nil
}

// tryDeliverToRemoteUser attempts AP delivery for a 1-on-1 chat message when
// the recipient is a remote user. Errors are logged and swallowed (DB commit
// is already done).
func (s *Service) tryDeliverToRemoteUser(msg *model.ChatMessage, fromUserID, toUserID string) {
	if s.deliverer == nil || s.userRepo == nil || s.renderer == nil || s.urls == nil {
		return
	}
	recipient, err := s.userRepo.FindByID(toUserID)
	if err != nil || recipient.IsLocal() {
		return
	}
	sender, err := s.userRepo.FindByID(fromUserID)
	if err != nil || !sender.IsLocal() {
		return
	}
	senderURI := s.urls.UserURI(sender.ID)
	recipientURI := ""
	if recipient.URI != nil {
		recipientURI = *recipient.URI
	}
	if recipientURI == "" {
		return
	}
	_ = s.repo.UpdateDeliveryStatus(msg.ID, true, false)
	cm := s.renderer.RenderChatMessage(msg, senderURI, recipientURI)
	body, err := json.Marshal(cm)
	if err != nil {
		_ = s.repo.UpdateDeliveryStatus(msg.ID, false, true)
		slog.Warn("chat: failed to marshal ChatMessage activity", "msgID", msg.ID, "err", err)
		return
	}
	if err := s.deliverer.DeliverToUser(sender.ID, recipient, body); err != nil {
		_ = s.repo.UpdateDeliveryStatus(msg.ID, false, true)
		slog.Warn("chat: failed to deliver ChatMessage to remote user", "msgID", msg.ID, "err", err)
		return
	}
	_ = s.repo.UpdateDeliveryStatus(msg.ID, false, false)
}

// CreateMessageViaAP persists a chat message received via ActivityPub from a
// remote user. The uri parameter is the activity's canonical ID. AP retries
// are common so URI-based dedup is performed (IngestNote と同じパターン).
func (s *Service) CreateMessageViaAP(ctx context.Context, uri string, fromUser *model.User, toUserID, text string) (*model.ChatMessage, error) {
	if fromUser == nil || toUserID == "" {
		return nil, ErrInvalidTarget
	}
	// AP retry による重複メッセージ作成を防ぐ
	if uri != "" {
		if existing, err := s.repo.FindMessageByURI(uri); err == nil {
			return existing, nil
		}
	}
	msg := &model.ChatMessage{
		ID:         s.idGen.Generate(time.Now()),
		FromUserID: fromUser.ID,
		ToUserID:   &toUserID,
	}
	if text != "" {
		msg.Text = &text
	}
	if uri != "" {
		msg.URI = &uri
	}
	if err := s.repo.CreateMessage(msg); err != nil {
		return nil, fmt.Errorf("create AP message: %w", err)
	}
	if s.publisher != nil {
		s.publisher.PublishUserMessage(ctx, fromUser.ID, toUserID, EventMessage, packMessage(msg))
	}
	return msg, nil
}

// CreateMessageToRoom persists a room-message row and fires the `message`
// event on the room topic so every subscribed member receives it.
func (s *Service) CreateMessageToRoom(ctx context.Context, fromUserID, roomID, text, fileID string) (*model.ChatMessage, error) {
	if fromUserID == "" || roomID == "" {
		return nil, ErrInvalidTarget
	}
	// ルームが存在し、送信者がメンバーであることを確認する。
	if _, err := s.repo.FindRoomByID(roomID); err != nil {
		return nil, ErrNotFound
	}
	isMember, err := s.IsRoomMember(fromUserID, roomID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrForbidden
	}
	msg := &model.ChatMessage{
		ID:         s.idGen.Generate(time.Now()),
		FromUserID: fromUserID,
		ToRoomID:   &roomID,
	}
	if text != "" {
		msg.Text = &text
	}
	if fileID != "" {
		msg.FileID = &fileID
	}
	if err := s.repo.CreateMessage(msg); err != nil {
		return nil, fmt.Errorf("create room message: %w", err)
	}
	if s.publisher != nil {
		s.publisher.PublishRoomMessage(ctx, roomID, EventMessage, packMessage(msg))
	}
	// Room 宛も同様に、sender 以外の全 member (owner 含む) の main に
	// newChatMessage を emit する。Misskey 本家 ChatService
	// .createMessageToRoom と同じ fan-out 方針。
	s.emitRoomNewChatMessage(roomID, fromUserID, msg)
	return msg, nil
}

// emitRoomNewChatMessage fans out `newChatMessage` to every room member
// except the sender. Room membership 列挙失敗時は警告ログを残して emit を
// スキップする (message 自体の作成は成功として返される前に呼ばれる)。
func (s *Service) emitRoomNewChatMessage(roomID, fromUserID string, msg *model.ChatMessage) {
	if s.mainStreamPublisher == nil {
		return
	}
	room, err := s.repo.FindRoomByID(roomID)
	if err != nil {
		slog.Warn("chat: FindRoomByID failed, skipping newChatMessage emit", "roomID", roomID, "err", err)
		return
	}
	members, err := s.repo.ListMembersByRoom(roomID)
	if err != nil {
		slog.Warn("chat: ListMembersByRoom failed, skipping newChatMessage emit", "roomID", roomID, "err", err)
		return
	}
	packed := packMessage(msg)
	// 重複 emit を防ぐため set 相当で ID を管理する (membership + owner)。
	emitted := make(map[string]bool, len(members)+1)
	if room.OwnerID != fromUserID {
		emitted[room.OwnerID] = true
	}
	for _, m := range members {
		if m.UserID == fromUserID || emitted[m.UserID] {
			continue
		}
		emitted[m.UserID] = true
	}
	for uid := range emitted {
		s.mainStreamPublisher.PublishMainEvent(uid, "newChatMessage", packed)
	}
}

// DeleteMessage removes a message and fans out a `deleted` event on the
// appropriate topic (user DM or room). Only the author may delete.
func (s *Service) DeleteMessage(ctx context.Context, userID, messageID string) error {
	msg, err := s.repo.FindMessageByID(messageID)
	if err != nil {
		return ErrNotFound
	}
	if msg.FromUserID != userID {
		return ErrForbidden
	}
	if err := s.repo.DeleteMessage(messageID); err != nil {
		return fmt.Errorf("delete message: %w", err)
	}
	if s.publisher != nil {
		body := map[string]any{"id": messageID}
		if msg.ToRoomID != nil {
			s.publisher.PublishRoomMessage(ctx, *msg.ToRoomID, EventDeleted, body)
		} else if msg.ToUserID != nil {
			s.publisher.PublishUserMessage(ctx, msg.FromUserID, *msg.ToUserID, EventDeleted, body)
		}
	}
	return nil
}

// UpdateMessage edits a message text and fires `edited`. Author-only.
func (s *Service) UpdateMessage(ctx context.Context, userID, messageID, text string) (*model.ChatMessage, error) {
	msg, err := s.repo.FindMessageByID(messageID)
	if err != nil {
		return nil, ErrNotFound
	}
	if msg.FromUserID != userID {
		return nil, ErrForbidden
	}
	msg.Text = &text
	if err := s.repo.UpdateMessage(msg); err != nil {
		return nil, fmt.Errorf("update message: %w", err)
	}
	if s.publisher != nil {
		body := packMessage(msg)
		if msg.ToRoomID != nil {
			s.publisher.PublishRoomMessage(ctx, *msg.ToRoomID, EventEdited, body)
		} else if msg.ToUserID != nil {
			s.publisher.PublishUserMessage(ctx, msg.FromUserID, *msg.ToUserID, EventEdited, body)
		}
	}
	return msg, nil
}

// MarkReadByMessageID records that userID has read messageID. Misskey 本家は
// 既読を redis cache のみで持ち配信しないが、マルチ端末/タブ同期のため軽微な
// 拡張として `read` イベントを発行する。
func (s *Service) MarkReadByMessageID(ctx context.Context, userID, messageID string) error {
	msg, err := s.repo.FindMessageByID(messageID)
	if err != nil {
		return ErrNotFound
	}
	if err := s.repo.MarkRead(userID, messageID); err != nil {
		return fmt.Errorf("mark read: %w", err)
	}
	if s.publisher != nil {
		body := map[string]any{"id": messageID, "userId": userID}
		if msg.ToRoomID != nil {
			s.publisher.PublishRoomMessage(ctx, *msg.ToRoomID, EventRead, body)
		} else if msg.ToUserID != nil {
			// DM の場合、両端末に既読状態を通知するため双方向へ送る。
			s.publisher.PublishUserMessage(ctx, msg.FromUserID, *msg.ToUserID, EventRead, body)
		}
	}
	return nil
}

// --- Queries used by the WebSocket channel for membership / auth checks ---

// IsRoomMember reports whether userID is a member of roomID. The room owner
// is implicitly a member even if no explicit chat_room_membership row exists.
func (s *Service) IsRoomMember(userID, roomID string) (bool, error) {
	room, err := s.repo.FindRoomByID(roomID)
	if err == nil && room.OwnerID == userID {
		return true, nil
	}
	if _, err := s.repo.FindMembership(userID, roomID); err == nil {
		return true, nil
	}
	return false, nil
}

// packMessage produces a JSON-serializable projection of a ChatMessage for
// outbound WebSocket events. フロント互換のため本家 ChatMessageLite と同じ
// 主要フィールドを含める。
func packMessage(msg *model.ChatMessage) map[string]any {
	out := map[string]any{
		"id":         msg.ID,
		"fromUserId": msg.FromUserID,
	}
	if msg.Text != nil {
		out["text"] = *msg.Text
	}
	if msg.ToUserID != nil {
		out["toUserId"] = *msg.ToUserID
	}
	if msg.ToRoomID != nil {
		out["toRoomId"] = *msg.ToRoomID
	}
	if msg.FileID != nil {
		out["fileId"] = *msg.FileID
	}
	if msg.URI != nil {
		out["uri"] = *msg.URI
	}
	if msg.Reads != nil {
		out["reads"] = []string(msg.Reads)
	}
	if msg.Reactions != nil {
		out["reactions"] = []string(msg.Reactions)
	}
	return out
}
