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
// Like / Undo Like activities to the inbox of remote note authors.
//
// 配信ルール:
//   - reactor がローカル & target の作者がリモート → Like を作者へ送る
//   - reactor がリモート → 配信不要 (どこかからの取り込み済みリアクション)
//   - target 作者がローカル → 配信不要 (受け側で fan-out する)
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

// OnReactionAdded emits a Like activity to the remote note author.
func (h *ReactionDeliveryHook) OnReactionAdded(reactor *model.User, target *model.Note, reaction string) {
	author, ok := h.resolveTargetAuthor(reactor, target)
	if !ok {
		return
	}
	like := h.buildLike(reactor, target, reaction)
	body, _ := json.Marshal(like)
	if err := h.deliver.DeliverToUser(reactor.ID, author, body); err != nil {
		slog.Warn("reaction delivery: like failed",
			"reactor", reactor.ID, "noteId", target.ID, "err", err)
	}
}

// OnReactionRemoved emits an Undo Like activity to the remote note author.
func (h *ReactionDeliveryHook) OnReactionRemoved(reactor *model.User, target *model.Note, reaction string) {
	author, ok := h.resolveTargetAuthor(reactor, target)
	if !ok {
		return
	}
	like := h.buildLike(reactor, target, reaction)
	undo := h.renderer.RenderUndoLike(reactor, like)
	body, _ := json.Marshal(undo)
	if err := h.deliver.DeliverToUser(reactor.ID, author, body); err != nil {
		slog.Warn("reaction delivery: undo like failed",
			"reactor", reactor.ID, "noteId", target.ID, "err", err)
	}
}

// resolveTargetAuthor returns the remote target author user, or false if no
// delivery is required.
func (h *ReactionDeliveryHook) resolveTargetAuthor(reactor *model.User, target *model.Note) (*model.User, bool) {
	if reactor == nil || target == nil {
		return nil, false
	}
	if !reactor.IsLocal() {
		return nil, false
	}
	author, err := h.userRepo.FindByID(target.UserID)
	if err != nil {
		slog.Warn("reaction delivery: author not found",
			"noteId", target.ID, "userId", target.UserID, "err", err)
		return nil, false
	}
	if author.IsLocal() {
		return nil, false
	}
	return author, true
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
