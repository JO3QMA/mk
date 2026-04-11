package webhook

import (
	"encoding/json"

	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
)

// NoteCreateHook wraps Service to implement the WebhookHook interface
// expected by core/note.CreateService. It fans an `OnNoteCreated` callback
// into the appropriate user webhook events (note / reply / renote / mention).
//
// 発火ルール:
//   - 作者に対して常に `note` を発火
//   - reply の場合は親ノートの投稿者に `reply` を発火
//   - renote の場合は renote 対象の投稿者に `renote` を発火
//   - mention の場合は mention された各ユーザーに `mention` を発火
type NoteCreateHook struct {
	svc   *Service
	idGen id.Generator
}

// NewNoteCreateHook constructs a NoteCreateHook.
func NewNoteCreateHook(svc *Service, idGen id.Generator) *NoteCreateHook {
	return &NoteCreateHook{svc: svc, idGen: idGen}
}

// OnNoteCreated fires note / reply / renote / mention webhook events. 失敗は
// すべて svc.DispatchUser 内でログに留まる。
func (h *NoteCreateHook) OnNoteCreated(note *model.Note, author *model.User, replyTarget, renoteTarget *model.Note) {
	if h == nil || h.svc == nil || note == nil || author == nil {
		return
	}
	body := map[string]any{"note": packNote(note, author, h.idGen)}

	// 作者本人のwebhook
	h.svc.DispatchUser(author.ID, EventNote, body)

	// reply: 親ノート投稿者
	if replyTarget != nil && replyTarget.UserID != author.ID {
		h.svc.DispatchUser(replyTarget.UserID, EventReply, body)
	}
	// renote: 対象ノート投稿者
	if renoteTarget != nil && renoteTarget.UserID != author.ID {
		h.svc.DispatchUser(renoteTarget.UserID, EventRenote, body)
	}
	// mention: 本文から抽出されたユーザー ID ごとに配信
	for _, mentionID := range note.Mentions {
		if mentionID == "" || mentionID == author.ID {
			continue
		}
		h.svc.DispatchUser(mentionID, EventMention, body)
	}
}

// ReactionCreateHook implements the reaction WebhookHook interface.
type ReactionCreateHook struct {
	svc   *Service
	idGen id.Generator
}

// NewReactionCreateHook constructs a ReactionCreateHook.
func NewReactionCreateHook(svc *Service, idGen id.Generator) *ReactionCreateHook {
	return &ReactionCreateHook{svc: svc, idGen: idGen}
}

// OnReactionCreated fires the `reaction` webhook event on the note author.
func (h *ReactionCreateHook) OnReactionCreated(note *model.Note, reactor *model.User, reaction string) {
	if h == nil || h.svc == nil || note == nil || reactor == nil {
		return
	}
	body := map[string]any{
		"note":     packNote(note, note.User, h.idGen),
		"userId":   reactor.ID,
		"reaction": reaction,
	}
	h.svc.DispatchUser(note.UserID, EventReaction, body)
}

// FollowingHook implements the following WebhookHook interface.
type FollowingHook struct {
	svc *Service
}

// NewFollowingHook constructs a FollowingHook.
func NewFollowingHook(svc *Service) *FollowingHook {
	return &FollowingHook{svc: svc}
}

// OnFollow fires the `follow` event on the follower's webhooks.
func (h *FollowingHook) OnFollow(follower, followee *model.User) {
	if h == nil || h.svc == nil || follower == nil || followee == nil {
		return
	}
	h.svc.DispatchUser(follower.ID, EventFollow, map[string]any{"user": packUser(followee)})
}

// OnUnfollow fires the `unfollow` event on the follower's webhooks.
func (h *FollowingHook) OnUnfollow(follower, followee *model.User) {
	if h == nil || h.svc == nil || follower == nil || followee == nil {
		return
	}
	h.svc.DispatchUser(follower.ID, EventUnfollow, map[string]any{"user": packUser(followee)})
}

// OnFollowed fires the `followed` event on the followee's webhooks.
func (h *FollowingHook) OnFollowed(follower, followee *model.User) {
	if h == nil || h.svc == nil || follower == nil || followee == nil {
		return
	}
	h.svc.DispatchUser(followee.ID, EventFollowed, map[string]any{"user": packUser(follower)})
}

// SignupHook implements the signup WebhookHook interface, firing the
// `userCreated` system webhook event on new user creation.
type SignupHook struct {
	svc *Service
}

// NewSignupHook constructs a SignupHook.
func NewSignupHook(svc *Service) *SignupHook {
	return &SignupHook{svc: svc}
}

// OnUserCreated fires the `userCreated` system webhook event.
func (h *SignupHook) OnUserCreated(user *model.User) {
	if h == nil || h.svc == nil || user == nil {
		return
	}
	h.svc.DispatchSystem(SystemEventUserCreated, packUser(user))
}

// packNote turns a model.Note into a generic map matching entity.NoteEntity.
// 既存の entity.PackNote を JSON 経由で map[string]any に変換する。
// Note/User/IDGen は呼び出し元でnil-check済みの前提。json.Marshal/Unmarshal は
// 正常値に対して失敗しないため戻り値は常に非nil。
func packNote(note *model.Note, author *model.User, idGen id.Generator) map[string]any {
	if note.User == nil && author != nil {
		clone := *note
		clone.User = author
		note = &clone
	}
	packed := entity.PackNote(note, idGen)
	raw, _ := json.Marshal(packed)
	out := map[string]any{}
	_ = json.Unmarshal(raw, &out)
	return out
}

// packUser turns a model.User into a generic map matching entity.UserLite.
// 呼び出し元で u != nil は保証済み。
func packUser(u *model.User) map[string]any {
	packed := entity.PackUserLite(u)
	raw, _ := json.Marshal(packed)
	out := map[string]any{}
	_ = json.Unmarshal(raw, &out)
	return out
}
