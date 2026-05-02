package federation

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/shiroha-a/mk/internal/activitypub"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

// ReactionDeliveryHook implements core/reaction.FederationHook by emitting
// Like / Undo Like activities so the reaction reaches every instance that
// observed the original note.
//
// 配信ルール (Misskey TS の ReactionService.create と一致 / drop-in 互換):
//   - reactor がローカル & note が localOnly でない:
//   - note 作者が remote → 作者の inbox に DirectRecipe
//   - visibility public / home / followers → reactor の remote followers
//     に fanout
//   - visibility specified → visible な remote user 全員に DirectRecipe
//   - reactor がリモート → 配信不要 (どこかからの取り込み済みリアクション)
//   - localOnly note → 配信不要
//
// note 作者がローカル user の場合でも followers fanout は実行する。これに
// より「自分の public note に自分でリアクションした」場合に remote
// followers のタイムラインに反映される (#636)。
type ReactionDeliveryHook struct {
	deliver  *DeliverService
	renderer *activitypub.Renderer
	urls     *activitypub.URLBuilder
	idGen    id.Generator
	userRepo repository.UserRepository
}

// NewReactionDeliveryHook constructs a ReactionDeliveryHook.
func NewReactionDeliveryHook(
	deliver *DeliverService,
	renderer *activitypub.Renderer,
	urls *activitypub.URLBuilder,
	idGen id.Generator,
	userRepo repository.UserRepository,
) *ReactionDeliveryHook {
	return &ReactionDeliveryHook{
		deliver:  deliver,
		renderer: renderer,
		urls:     urls,
		idGen:    idGen,
		userRepo: userRepo,
	}
}

// OnReactionAdded emits a Like activity to all instances that should observe
// the reaction (note author + reactor's followers / specified recipients).
func (h *ReactionDeliveryHook) OnReactionAdded(reactor *model.User, target *model.Note, reaction string) {
	if !h.shouldDeliver(reactor, target) {
		return
	}
	like := h.buildLike(reactor, target, reaction)
	body, _ := json.Marshal(like)
	h.fanout(reactor, target, body, "like")
}

// OnReactionRemoved emits an Undo Like activity with the same recipient set.
func (h *ReactionDeliveryHook) OnReactionRemoved(reactor *model.User, target *model.Note, reaction string) {
	if !h.shouldDeliver(reactor, target) {
		return
	}
	like := h.buildLike(reactor, target, reaction)
	undo := h.renderer.RenderUndoLike(reactor, like)
	body, _ := json.Marshal(undo)
	h.fanout(reactor, target, body, "undo like")
}

// shouldDeliver returns false when the reaction is a no-op for federation
// (remote reactor, localOnly note, missing args).
func (h *ReactionDeliveryHook) shouldDeliver(reactor *model.User, target *model.Note) bool {
	if reactor == nil || target == nil {
		return false
	}
	if !reactor.IsLocal() {
		return false
	}
	if target.LocalOnly {
		return false
	}
	return true
}

// fanout collects DirectRecipe inboxes (note author + specified recipients)
// and follower inboxes per the note visibility, then enqueues the activity
// body to each. visibility に応じて TS の DeliverManager と同じ recipient
// 集合になるよう揃える。
//
// kind は失敗ログ識別用 ("like" / "undo like")。
func (h *ReactionDeliveryHook) fanout(reactor *model.User, target *model.Note, body []byte, kind string) {
	directInboxes := h.directInboxes(target)
	if len(directInboxes) > 0 {
		if err := h.deliver.DeliverActivity(reactor.ID, body, directInboxes); err != nil {
			slog.Warn("reaction delivery: direct failed",
				"kind", kind, "reactor", reactor.ID, "noteId", target.ID, "err", err)
		}
	}
	if h.shouldFanoutToFollowers(target) {
		// DeliverToFollowers が内部で followingRepo から remote follower
		// inbox を fetch して DeliverActivity に渡す。directInboxes と
		// 重複する inbox があった場合は別 batch なので enqueue が 2 回
		// 走るが、Like ID 同一なので相手側で idempotent (already-reacted
		// 扱い)。TS DeliverManager のように完全 dedup したい場合は
		// followingRepo を hook 側に持つ必要があり、今回はコストに見合
		// わないので分離 batch のまま許容する。
		if err := h.deliver.DeliverToFollowers(reactor.ID, body); err != nil {
			slog.Warn("reaction delivery: followers fanout failed",
				"kind", kind, "reactor", reactor.ID, "noteId", target.ID, "err", err)
		}
	}
}

// directInboxes returns the explicit DirectRecipe inboxes: note author (if
// remote) + visible specified users (if remote). 作者が local の場合は省く
// (TS と同じ条件)。
func (h *ReactionDeliveryHook) directInboxes(target *model.Note) []string {
	var inboxes []string
	if author, err := h.userRepo.FindByID(target.UserID); err == nil {
		if !author.IsLocal() {
			if inbox := preferredInbox(author); inbox != "" {
				inboxes = append(inboxes, inbox)
			}
		}
	} else {
		slog.Warn("reaction delivery: author not found",
			"noteId", target.ID, "userId", target.UserID, "err", err)
	}
	if target.Visibility == model.NoteVisibilitySpecified {
		for _, uid := range target.VisibleUserIDs {
			u, err := h.userRepo.FindByID(uid)
			if err != nil || u == nil {
				continue
			}
			if u.IsLocal() {
				continue
			}
			if inbox := preferredInbox(u); inbox != "" {
				inboxes = append(inboxes, inbox)
			}
		}
	}
	return inboxes
}

// shouldFanoutToFollowers reports whether the note visibility triggers
// follower fanout. specified の場合は DirectRecipe で済ませる (TS と一致)。
func (h *ReactionDeliveryHook) shouldFanoutToFollowers(target *model.Note) bool {
	switch target.Visibility {
	case model.NoteVisibilityPublic, model.NoteVisibilityHome, model.NoteVisibilityFollowers:
		return true
	default:
		return false
	}
}

// buildLike constructs the Like activity used both for create and undo paths.
func (h *ReactionDeliveryHook) buildLike(reactor *model.User, target *model.Note, reaction string) *activitypub.Like {
	targetURI := h.urls.NoteURI(target.ID)
	if target.URI != nil && *target.URI != "" {
		targetURI = *target.URI
	}
	likeID := h.urls.UserURI(reactor.ID) + "/likes/" + h.idGen.Generate(time.Now())
	return h.renderer.RenderLike(reactor, targetURI, reaction, likeID)
}
