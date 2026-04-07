package federation

import (
	"encoding/json"
	"log/slog"

	"github.com/shiroha-a/mk/internal/activitypub"
	"github.com/shiroha-a/mk/internal/model"
)

// FollowingDeliveryHook implements core/following.FederationHook by emitting
// Follow / Undo Follow / Accept activities to remote inboxes.
//
// 配信は次のケースだけ行う:
//   - ローカル user が remote user をフォロー → Follow を remote inbox に送信
//   - ローカル user が remote user をアンフォロー → Undo Follow を送信
//   - ローカル user が remote user の follow request を受諾 → Accept を送信
//     (req.FollowerID がリモート、followee がローカル)
type FollowingDeliveryHook struct {
	deliver  *DeliverService
	renderer *activitypub.Renderer
	urls     *activitypub.URLBuilder
}

// NewFollowingDeliveryHook constructs a FollowingDeliveryHook.
func NewFollowingDeliveryHook(deliver *DeliverService, renderer *activitypub.Renderer, urls *activitypub.URLBuilder) *FollowingDeliveryHook {
	return &FollowingDeliveryHook{deliver: deliver, renderer: renderer, urls: urls}
}

// OnLocalFollowed sends a Follow activity to a remote followee.
func (h *FollowingDeliveryHook) OnLocalFollowed(follower, followee *model.User) {
	if !shouldDeliverFollow(follower, followee) {
		return
	}
	follow := h.renderer.RenderFollow(follower.ID, *followee.URI)
	body, _ := json.Marshal(follow)
	if err := h.deliver.DeliverToUser(follower.ID, followee, body); err != nil {
		slog.Warn("following delivery: follow failed",
			"follower", follower.ID, "followee", followee.ID, "err", err)
	}
}

// OnLocalUnfollowed sends an Undo Follow activity to a remote followee.
func (h *FollowingDeliveryHook) OnLocalUnfollowed(follower, followee *model.User) {
	if !shouldDeliverFollow(follower, followee) {
		return
	}
	follow := h.renderer.RenderFollow(follower.ID, *followee.URI)
	undo := &activitypub.Undo{
		Activity: activitypub.Activity{
			Object: activitypub.Object{
				Type: "Undo",
				ID:   follow.ID + "/undo",
			},
			Actor: follow.Actor,
		},
		Object: follow,
	}
	activitypub.AddContext(undo)
	body, _ := json.Marshal(undo)
	if err := h.deliver.DeliverToUser(follower.ID, followee, body); err != nil {
		slog.Warn("following delivery: unfollow failed",
			"follower", follower.ID, "followee", followee.ID, "err", err)
	}
}

// OnLocalFollowAccepted sends an Accept activity to a remote follower.
// 引数: follower がリモート、followee がローカル (Accept の actor)。
func (h *FollowingDeliveryHook) OnLocalFollowAccepted(follower, followee *model.User) {
	if follower == nil || followee == nil {
		return
	}
	if follower.IsLocal() || !followee.IsLocal() {
		// follower がローカル / followee がリモート の組み合わせは AP配信不要。
		return
	}
	if follower.URI == nil || *follower.URI == "" {
		return
	}
	// 元の Follow を再構成する: actor は remote follower の URI、
	// object はローカル followee の正規URI。
	localFolloweeURI := h.urls.UserURI(followee.ID)
	follow := &activitypub.Follow{
		Activity: activitypub.Activity{
			Object: activitypub.Object{Type: "Follow"},
			Actor:  *follower.URI,
		},
		Object: localFolloweeURI,
	}
	accept := h.renderer.RenderAccept(followee.ID, follow)
	body, _ := json.Marshal(accept)
	if err := h.deliver.DeliverToUser(followee.ID, follower, body); err != nil {
		slog.Warn("following delivery: accept failed",
			"follower", follower.ID, "followee", followee.ID, "err", err)
	}
}

// shouldDeliverFollow reports whether a Follow / Undo Follow should be sent.
// 必須条件: follower がローカル, followee がリモートで URI を持っている。
func shouldDeliverFollow(follower, followee *model.User) bool {
	if follower == nil || followee == nil {
		return false
	}
	if !follower.IsLocal() {
		return false
	}
	if followee.IsLocal() {
		return false
	}
	if followee.URI == nil || *followee.URI == "" {
		return false
	}
	return true
}
