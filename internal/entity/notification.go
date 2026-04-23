package entity

// Note: this file intentionally imports `internal/core/notification` for
// the Notification struct definition, which is the only exception to the
// entity→{model,misc}-only convention in this package. Moving the struct
// to `internal/model` would touch 20+ call sites; the direct import keeps
// the packer API ergonomic and is limited to this one file. Callers that
// want to avoid the transitive dependency should pass primitives instead.

import (
	"github.com/shiroha-a/mk/internal/core/notification"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
)

// PackNotification converts a Notification record into the map shape
// emitted over the `main` WebSocket channel and returned by
// /api/i/notifications. Matches Misskey's NotificationEntityService.pack
// output: id / createdAt / type / userId / user (PackUserLite) / note
// (PackNote) / reaction / choice. Optional user/note arguments are only
// embedded when non-nil so the caller can pre-fetch once and re-use.
//
// instLookup / emojiLookup を渡すと top-level user と note.user に
// Instance / 絵文字 URL を埋める。nil なら no-op リゾルバとして扱われ、
// 従来互換の挙動になる (#415 notifications page InstanceTicker)。
func PackNotification(n *notification.Notification, user *model.User, note *model.Note, idGen id.Generator, instLookup InstanceLookup, emojiLookup EmojiLookup) map[string]any {
	if n == nil {
		return nil
	}
	const tsFormat = "2006-01-02T15:04:05.000Z"
	out := map[string]any{
		"id":        n.ID,
		"createdAt": n.CreatedAt.UTC().Format(tsFormat),
		"type":      string(n.Type),
	}
	if n.NotifierID != "" {
		// TS本家はnotifierIdを"userId"として返す。
		out["userId"] = n.NotifierID
		if user != nil {
			out["user"] = packUserLiteWithResolvers(user, instLookup, emojiLookup)
		}
	}
	if n.NoteID != "" {
		// reply/mention/reaction/renote/quote/pollEnded等、noteを要する
		// 通知タイプ向け。REST API互換のため noteId は常に含める。
		// 加えて note object が fetch できていればpacked Noteも入れる。
		// PackNoteWithInstance で note.user / Renote/Reply の embed にも
		// Instance / 絵文字を適用する。
		out["noteId"] = n.NoteID
		if note != nil && idGen != nil {
			out["note"] = PackNoteWithInstance(note, idGen, instLookup, emojiLookup)
		}
	}
	if n.Reaction != "" {
		out["reaction"] = n.Reaction
	}
	if n.Choice != nil {
		out["choice"] = *n.Choice
	}
	// extraはTS本家では role/invitation/achievement 等の個別フィールド
	// として展開されるが、Go側はmap[string]anyで透過的にmergeする。
	// core keyと衝突する場合は extra 側をskip (packerの契約を壊さない)。
	for k, v := range n.Extra {
		if _, collides := out[k]; collides {
			continue
		}
		out[k] = v
	}
	return out
}

// packUserLiteWithResolvers packs a model.User and applies optional Instance /
// emoji resolution so that top-level user surfaces (notification userId / user)
// carry the same InstanceTicker metadata as embedded note authors. nil lookups
// leave Instance / Emojis empty (same as plain PackUserLite).
func packUserLiteWithResolvers(u *model.User, instLookup InstanceLookup, emojiLookup EmojiLookup) UserLite {
	lite := PackUserLite(u)
	if u == nil {
		return lite
	}
	instResolver := NewInstanceResolver(instLookup, u)
	instResolver.FillUserLite(&lite)
	// 絵文字は note.User の User.Emojis を引く経路と同じ interface を再利用。
	// 合成 note に User だけ詰めた shell を渡すと NewEmojiResolver は
	// user.Emojis から unique name を拾って一括 fetch してくれる。
	emojiResolver := NewEmojiResolver(emojiLookup, []*model.Note{{User: u}})
	emojiResolver.PopulateUserEmojis(u, &lite)
	return lite
}
