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
func PackNotification(n *notification.Notification, user *model.User, note *model.Note, idGen id.Generator) map[string]any {
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
			out["user"] = PackUserLite(user)
		}
	}
	if n.NoteID != "" {
		// reply/mention/reaction/renote/quote/pollEnded等、noteを要する
		// 通知タイプ向け。REST API互換のため noteId は常に含める。
		// 加えて note object が fetch できていればpacked Noteも入れる。
		out["noteId"] = n.NoteID
		if note != nil && idGen != nil {
			out["note"] = PackNote(note, idGen)
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
