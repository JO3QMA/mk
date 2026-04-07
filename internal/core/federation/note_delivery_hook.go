package federation

import (
	"encoding/json"
	"log/slog"

	"github.com/shiroha-a/mk/internal/activitypub"
	corenote "github.com/shiroha-a/mk/internal/core/note"
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
//
// pure renote (text/cw/file/poll を伴わない renote) は Create ではなく
// Announce activity として配信する。
type NoteDeliveryHook struct {
	deliver  *DeliverService
	renderer *activitypub.Renderer
	urls     *activitypub.URLBuilder
	idGen    id.Generator
	userRepo repository.UserRepository
	noteRepo repository.NoteRepository
}

// NewNoteDeliveryHook constructs a NoteDeliveryHook.
func NewNoteDeliveryHook(
	deliver *DeliverService,
	renderer *activitypub.Renderer,
	urls *activitypub.URLBuilder,
	idGen id.Generator,
	userRepo repository.UserRepository,
	noteRepo repository.NoteRepository,
) *NoteDeliveryHook {
	return &NoteDeliveryHook{
		deliver:  deliver,
		renderer: renderer,
		urls:     urls,
		idGen:    idGen,
		userRepo: userRepo,
		noteRepo: noteRepo,
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

	// 純粋なリノートは Create ではなく Announce として配信する。
	if corenote.IsPureRenote(note) {
		h.deliverAnnounce(note, author)
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

// deliverAnnounce renders an Announce activity for a pure renote and ships it
// to the renoter's followers.
func (h *NoteDeliveryHook) deliverAnnounce(note *model.Note, author *model.User) {
	target, err := h.noteRepo.FindByID(*note.RenoteID)
	if err != nil {
		slog.Warn("note delivery: renote target not found",
			"noteId", note.ID, "renoteId", *note.RenoteID, "err", err)
		return
	}
	targetURI := h.urls.NoteURI(target.ID)
	if target.URI != nil && *target.URI != "" {
		targetURI = *target.URI
	}
	announce := h.renderer.RenderAnnounce(author, note.ID, targetURI)
	body, _ := json.Marshal(announce)
	if err := h.deliver.DeliverToFollowers(author.ID, body); err != nil {
		slog.Warn("note delivery: announce failed",
			"noteId", note.ID, "err", err)
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
