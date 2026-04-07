package federation

import (
	"encoding/json"
	"log/slog"

	"github.com/shiroha-a/mk/internal/activitypub"
	"github.com/shiroha-a/mk/internal/model"
)

// NoteDeleteDeliveryHook implements core/note.DeleteFederationHook by emitting
// a Delete activity to the followers of the local author.
//
// 配信ルール:
//   - author がリモート → 配信不要 (リモート側で発火する)
//   - localOnly note → 配信不要
//   - それ以外 → フォロワー (リモート) に Delete を送る
type NoteDeleteDeliveryHook struct {
	deliver  *DeliverService
	renderer *activitypub.Renderer
	urls     *activitypub.URLBuilder
}

// NewNoteDeleteDeliveryHook constructs a NoteDeleteDeliveryHook.
func NewNoteDeleteDeliveryHook(
	deliver *DeliverService,
	renderer *activitypub.Renderer,
	urls *activitypub.URLBuilder,
) *NoteDeleteDeliveryHook {
	return &NoteDeleteDeliveryHook{deliver: deliver, renderer: renderer, urls: urls}
}

// OnNoteDeleted is invoked by NoteDeleteService once a note has been removed.
func (h *NoteDeleteDeliveryHook) OnNoteDeleted(author *model.User, note *model.Note) {
	if author == nil || note == nil {
		return
	}
	if !author.IsLocal() {
		return
	}
	if note.LocalOnly {
		return
	}

	noteURI := h.urls.NoteURI(note.ID)
	if note.URI != nil && *note.URI != "" {
		noteURI = *note.URI
	}
	del := h.renderer.RenderDelete(author, noteURI)
	body, _ := json.Marshal(del)
	if err := h.deliver.DeliverToFollowers(author.ID, body); err != nil {
		slog.Warn("note delete delivery failed",
			"noteId", note.ID, "err", err)
	}
}
