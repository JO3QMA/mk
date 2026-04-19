package notification

import (
	"context"
	"log/slog"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

// MuteChecker reports whether muter has muted mutee. パッケージ間の循環依存を
// 避けるためinterfaceで受け取る (実装は core/muting)。
type MuteChecker interface {
	IsMuted(muterID, muteeID string) (bool, error)
}

// WebPushPublisher enqueues a Web Push notification job for a user. パッケージ
// 間の循環依存を避けるため interface で受け取る (実装は core/webpush.Service)。
type WebPushPublisher interface {
	PushNotification(userID string, body map[string]any)
}

// NotePacker packs a note ID into a JSON-serializable representation matching
// the Misskey-packed note shape. Used for composing Web Push payloads without
// creating a cycle on the entity package.
type NotePacker interface {
	PackNoteByID(noteID string) (map[string]any, bool)
}

// UserPacker packs a user ID into a JSON-serializable representation matching
// the Misskey-packed user shape.
type UserPacker interface {
	PackUserByID(userID string) (map[string]any, bool)
}

// Hook implements the various NotificationHook interfaces exposed by other
// services. Single struct in order to share the underlying Service and
// userRepo dependencies.
type Hook struct {
	svc         *Service
	userRepo    repository.UserRepository
	muteChecker MuteChecker
	webpush     WebPushPublisher
	notePacker  NotePacker
	userPacker  UserPacker
}

// NewHook constructs a Hook bound to a NotificationService and userRepo.
// userRepo is used to look up host information when filtering remote users
// (リモートユーザーへの通知は不要なので除外する)。
func NewHook(svc *Service, userRepo repository.UserRepository) *Hook {
	return &Hook{svc: svc, userRepo: userRepo}
}

// SetMuteChecker attaches a MuteChecker. 通知前にmuteEEがmuterをmuteしている
// 場合は通知をスキップする (Misskey本家の挙動)。
func (h *Hook) SetMuteChecker(c MuteChecker) {
	h.muteChecker = c
}

// SetWebPushPublisher attaches a WebPushPublisher. 通知作成後にWeb Push
// 配信キューへenqueueするために使う。
func (h *Hook) SetWebPushPublisher(p WebPushPublisher) {
	h.webpush = p
}

// SetPackers attaches user/note packers used when composing Web Push payloads.
// Either or both may be nil; payload fields will be omitted accordingly.
func (h *Hook) SetPackers(u UserPacker, n NotePacker) {
	h.userPacker = u
	h.notePacker = n
}

// OnNoteCreated is called by note.CreateService after persisting a new note.
// Reply/Renote/Mention の通知を非同期に作成する。
func (h *Hook) OnNoteCreated(n *model.Note, author *model.User, replyTarget, renoteTarget *model.Note) {
	if n == nil || author == nil {
		return
	}
	ctx := context.Background()

	// reply: 親ノートの投稿者がローカルユーザーなら通知
	if replyTarget != nil && replyTarget.UserID != author.ID {
		h.notifyLocalUser(ctx, replyTarget.UserID, CreateInput{
			NotifieeID:     replyTarget.UserID,
			NotifierID:     author.ID,
			Type:           TypeReply,
			NoteID:         n.ID,
			NoteVisibility: string(n.Visibility),
		})
	}

	// renote / quote: 対象ノートの投稿者へ通知
	if renoteTarget != nil && renoteTarget.UserID != author.ID {
		t := TypeRenote
		if isQuote(n) {
			t = TypeQuote
		}
		h.notifyLocalUser(ctx, renoteTarget.UserID, CreateInput{
			NotifieeID:     renoteTarget.UserID,
			NotifierID:     author.ID,
			Type:           t,
			NoteID:         n.ID,
			NoteVisibility: string(n.Visibility),
		})
	}

	// mentions: note.Mentions はユーザーIDのリスト (またはレガシーのユーザー名)
	for _, idOrName := range n.Mentions {
		if idOrName == "" {
			continue
		}
		// まずIDとして解決を試みる。失敗したらユーザー名として解決する。
		mentionedID := idOrName
		if _, err := h.userRepo.FindByID(idOrName); err != nil {
			mentioned, err := h.userRepo.FindByUsernameLower(idOrName, nil)
			if err != nil {
				continue
			}
			mentionedID = mentioned.ID
		}
		if mentionedID == author.ID {
			continue
		}
		// reply先と同じユーザーには replyとmentionの両方を出さない
		if replyTarget != nil && replyTarget.UserID == mentionedID {
			continue
		}
		h.notifyLocalUser(ctx, mentionedID, CreateInput{
			NotifieeID:     mentionedID,
			NotifierID:     author.ID,
			Type:           TypeMention,
			NoteID:         n.ID,
			NoteVisibility: string(n.Visibility),
		})
	}
}

// OnFollowed records a follow notification on the followee's stream.
func (h *Hook) OnFollowed(followerID, followeeID string) {
	h.notifyLocalUser(context.Background(), followeeID, CreateInput{
		NotifieeID: followeeID,
		NotifierID: followerID,
		Type:       TypeFollow,
	})
}

// OnFollowRequested records a "follow request received" notification.
func (h *Hook) OnFollowRequested(followerID, followeeID string) {
	h.notifyLocalUser(context.Background(), followeeID, CreateInput{
		NotifieeID: followeeID,
		NotifierID: followerID,
		Type:       TypeReceiveFollowReq,
	})
}

// OnFollowAccepted records a notification on the requester's side.
func (h *Hook) OnFollowAccepted(followerID, followeeID string) {
	h.notifyLocalUser(context.Background(), followerID, CreateInput{
		NotifieeID: followerID,
		NotifierID: followeeID,
		Type:       TypeFollowRequestAccept,
	})
}

// OnReactionCreated records a reaction notification on the note author's stream.
func (h *Hook) OnReactionCreated(notifieeID, notifierID, noteID, reaction string) {
	h.notifyLocalUser(context.Background(), notifieeID, CreateInput{
		NotifieeID: notifieeID,
		NotifierID: notifierID,
		Type:       TypeReaction,
		NoteID:     noteID,
		Reaction:   reaction,
	})
}

// OnPollVote records a poll vote notification on the note author's stream.
func (h *Hook) OnPollVote(notifieeID, notifierID, noteID string, choice int) {
	c := choice
	h.notifyLocalUser(context.Background(), notifieeID, CreateInput{
		NotifieeID: notifieeID,
		NotifierID: notifierID,
		Type:       TypePollVote,
		NoteID:     noteID,
		Choice:     &c,
	})
}

// notifyLocalUser dispatches a notification only when the notifiee is a local
// user (host == nil). リモートユーザーへの通知はAP連合経由で送られるので
// ローカルストリームには入れない。Muteしているnotifierからの通知も抑制する。
func (h *Hook) notifyLocalUser(ctx context.Context, notifieeID string, in CreateInput) {
	if h.userRepo != nil {
		u, err := h.userRepo.FindByID(notifieeID)
		if err != nil {
			return
		}
		if u.Host != nil {
			return
		}
	}
	// notifiee がnotifierをmuteしている場合は通知をスキップする
	if h.muteChecker != nil && in.NotifierID != "" {
		if muted, err := h.muteChecker.IsMuted(notifieeID, in.NotifierID); err == nil && muted {
			return
		}
	}
	n, err := h.svc.Create(ctx, in)
	if err != nil {
		slog.Warn("notification create failed", "type", in.Type, "notifiee", notifieeID, "err", err)
		return
	}
	// 通知の永続化に成功したらWeb Push配信キューへ投入する。
	// packerが未設定でもtype/id/userIdは最低限埋まるので、sw.js側の24h
	// 破棄チェックとユーザー判定は成立する。
	if h.webpush != nil && n != nil {
		h.webpush.PushNotification(notifieeID, h.buildPushBody(n))
	}
}

// buildPushBody converts a persisted Notification into a map matching
// the Misskey `Packed<'Notification'>` shape. Missing fields are omitted so
// that sw.js gracefully falls back to defaults.
func (h *Hook) buildPushBody(n *Notification) map[string]any {
	body := map[string]any{
		"id":        n.ID,
		"createdAt": n.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
		"type":      string(n.Type),
	}
	if n.NotifierID != "" {
		body["userId"] = n.NotifierID
		if h.userPacker != nil {
			if packed, ok := h.userPacker.PackUserByID(n.NotifierID); ok {
				body["user"] = packed
			}
		}
	}
	if n.NoteID != "" {
		body["noteId"] = n.NoteID
		if h.notePacker != nil {
			if packed, ok := h.notePacker.PackNoteByID(n.NoteID); ok {
				body["note"] = packed
			}
		}
	}
	if n.Reaction != "" {
		body["reaction"] = n.Reaction
	}
	if n.Choice != nil {
		body["choice"] = *n.Choice
	}
	return body
}

// isQuote reports whether the note is a quote renote (renote with text/cw/files/poll).
func isQuote(n *model.Note) bool {
	if n.RenoteID == nil {
		return false
	}
	if n.Text != nil && *n.Text != "" {
		return true
	}
	if n.CW != nil && *n.CW != "" {
		return true
	}
	if len(n.FileIDs) > 0 {
		return true
	}
	if n.HasPoll {
		return true
	}
	return false
}
