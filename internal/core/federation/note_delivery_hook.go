package federation

import (
	"encoding/json"
	"log/slog"

	"github.com/shiroha-a/mk/internal/activitypub"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

// NoteDeliveryHook implements core/note.FederationHook by rendering a Create
// activity and dispatching it through DeliverService.
//
// 配信先は visibility に応じて変える:
//   - public/home/followers: フォロワーへ
//   - specified: visibleUserIds の中のリモートユーザーへ
//   - localOnly: 何もしない
type NoteDeliveryHook struct {
	deliver  *DeliverService
	renderer *activitypub.Renderer
	idGen    id.Generator
	userRepo repository.UserRepository
}

// NewNoteDeliveryHook constructs a NoteDeliveryHook.
func NewNoteDeliveryHook(
	deliver *DeliverService,
	renderer *activitypub.Renderer,
	idGen id.Generator,
	userRepo repository.UserRepository,
) *NoteDeliveryHook {
	return &NoteDeliveryHook{
		deliver:  deliver,
		renderer: renderer,
		idGen:    idGen,
		userRepo: userRepo,
	}
}

// OnNoteCreated is invoked by NoteCreateService once a note has been
// persisted. リモートユーザーが投稿した note は (取り込み経由で生成された場合
// 含め) リモート発信なので連合配信しない。
func (h *NoteDeliveryHook) OnNoteCreated(note *model.Note, author *model.User) {
	if note == nil || author == nil {
		return
	}
	if !author.IsLocal() {
		return
	}
	if note.LocalOnly {
		return
	}

	create := h.renderer.RenderCreate(note, h.idGen)
	// renderer 由来の Create は string/[]string/Note (string fields のみ) で
	// 構成されるため json.Marshal が失敗するケースは存在しない。
	body, _ := json.Marshal(create)

	switch note.Visibility {
	case model.NoteVisibilityPublic, model.NoteVisibilityHome, model.NoteVisibilityFollowers:
		if err := h.deliver.DeliverToFollowers(author.ID, body); err != nil {
			slog.Warn("note delivery: deliver to followers failed",
				"noteId", note.ID, "err", err)
		}
	case model.NoteVisibilitySpecified:
		h.deliverToSpecified(author, note, body)
	}
}

// deliverToSpecified looks up each visibleUserId and routes to the recipient
// if remote.
func (h *NoteDeliveryHook) deliverToSpecified(author *model.User, note *model.Note, body []byte) {
	for _, uid := range note.VisibleUserIDs {
		recipient, err := h.userRepo.FindByID(uid)
		if err != nil {
			slog.Warn("note delivery: visible user not found",
				"noteId", note.ID, "userId", uid, "err", err)
			continue
		}
		if err := h.deliver.DeliverToUser(author.ID, recipient, body); err != nil {
			slog.Warn("note delivery: deliver to user failed",
				"noteId", note.ID, "userId", uid, "err", err)
		}
	}
}
