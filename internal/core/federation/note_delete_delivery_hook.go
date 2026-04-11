package federation

import (
	"encoding/json"
	"log/slog"

	"github.com/shiroha-a/mk/internal/activitypub"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

// NoteDeleteDeliveryHook implements core/note.DeleteFederationHook by emitting
// a Delete activity to the followers of the local author.
//
// 配信ルール:
//   - author がリモート → 配信不要 (リモート側で発火する)
//   - localOnly note → 配信不要
//   - それ以外 → フォロワー (リモート) + 既知のリモートインスタンス全体
//     (sharedInbox 単位) に Delete を送る。ap/show などで pull 済みの
//     インスタンスにも反映させるためフォロワーに加えブロードキャストする。
type NoteDeleteDeliveryHook struct {
	deliver  *DeliverService
	renderer *activitypub.Renderer
	urls     *activitypub.URLBuilder
	userRepo repository.UserRepository
}

// NewNoteDeleteDeliveryHook constructs a NoteDeleteDeliveryHook.
func NewNoteDeleteDeliveryHook(
	deliver *DeliverService,
	renderer *activitypub.Renderer,
	urls *activitypub.URLBuilder,
) *NoteDeleteDeliveryHook {
	return &NoteDeleteDeliveryHook{deliver: deliver, renderer: renderer, urls: urls}
}

// SetUserRepo wires a user repository used by the hook to look up remote
// inboxes for broadcast delivery. nil 渡しはフォロワーのみへの配信に戻る。
func (h *NoteDeleteDeliveryHook) SetUserRepo(r repository.UserRepository) {
	h.userRepo = r
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
	// Public / Home などは既知のリモートインスタンス全体にも Delete を送る。
	// フォロワー以外が ap/show 経由で note を取り込んでいるケースを補償する。
	if h.userRepo == nil {
		return
	}
	switch note.Visibility {
	case model.NoteVisibilityPublic, model.NoteVisibilityHome:
	default:
		return
	}
	inboxes, err := h.userRepo.ListRemoteInboxes()
	if err != nil {
		slog.Warn("note delete delivery: list remote inboxes failed",
			"noteId", note.ID, "err", err)
		return
	}
	if len(inboxes) == 0 {
		return
	}
	if err := h.deliver.DeliverActivity(author.ID, body, inboxes); err != nil {
		slog.Warn("note delete delivery: broadcast failed",
			"noteId", note.ID, "err", err)
	}
}
