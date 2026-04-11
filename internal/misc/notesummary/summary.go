// Package notesummary produces a compact text summary of a packed note,
// mirroring Misskey's get-note-summary.ts.
// プッシュ通知のbody短縮など、ノート内容を1行テキストに圧縮する用途に使う。
package notesummary

import (
	"fmt"
	"strings"
)

// Get returns a compact single/multi-line summary for a packed note.
// The note argument is expected to be the JSON-decoded packed note
// (i.e. map[string]any), matching Misskey's `Packed<'Note'>` shape.
//
// 本家Misskeyの `packages/backend/src/misc/get-note-summary.ts` と同等の
// 挙動を保つこと。sw.js のnotification描画が前提としているフォーマット
// なので、崩すとフロント側で表示が壊れる可能性がある。
func Get(note map[string]any) string {
	if note == nil {
		return ""
	}
	if v, ok := note["deletedAt"]; ok && v != nil {
		return "(\u274C\u26D4)"
	}
	if hidden, ok := note["isHidden"].(bool); ok && hidden {
		return "(\u26D4)"
	}

	var summary string
	if cw, ok := note["cw"].(string); ok && cw != "" {
		summary = cw
	} else if text, ok := note["text"].(string); ok {
		summary = text
	}

	if files, ok := note["files"].([]any); ok && len(files) > 0 {
		summary += fmt.Sprintf(" (\U0001F4CE%d)", len(files))
	}
	if poll, ok := note["poll"]; ok && poll != nil {
		summary += " (\U0001F4CA)"
	}
	if replyID, ok := note["replyId"].(string); ok && replyID != "" {
		if reply, ok := note["reply"].(map[string]any); ok {
			summary += "\n\nRE: " + Get(reply)
		}
	}
	if renoteID, ok := note["renoteId"].(string); ok && renoteID != "" {
		if renote, ok := note["renote"].(map[string]any); ok {
			summary += "\n\nRN: " + Get(renote)
		}
	}
	return strings.TrimSpace(summary)
}
