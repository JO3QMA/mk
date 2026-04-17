package following

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	corefollowing "github.com/shiroha-a/mk/internal/core/following"
	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// Invalidate handles POST /api/following/invalidate.
//
// 認証ユーザーが自分のフォロワーのうち userId 指定の相手を強制的に外す。
// Misskey 本家互換で RequireAuth レベル (moderator 不要) のため、嫌がらせ
// フォロワーに対して本人がいつでも解除できる。
func (h *Handler) Invalidate(c echo.Context) error {
	me := middleware.GetUser(c)
	var req struct {
		UserID string `json:"userId"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return apierr.JSONInvalidParam(c)
	}
	if err := h.followingService.Unfollow(req.UserID, me.ID); err != nil {
		if errors.Is(err, corefollowing.ErrNotFollowing) {
			return c.JSON(http.StatusBadRequest, apierr.Error("NOT_FOLLOWING", "You are not followed by that user.", "5dbf82f5-c92b-40b1-87d1-6c8c0741fd09"))
		}
		return apierr.JSONInternalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}

// UpdateFollow handles POST /api/following/update.
//
// notify ("normal" / "none") / withReplies を呼び出しユーザーの指定 followee
// に対して更新する。
func (h *Handler) UpdateFollow(c echo.Context) error {
	me := middleware.GetUser(c)
	var req struct {
		UserID      string  `json:"userId"`
		Notify      *string `json:"notify"`
		WithReplies *bool   `json:"withReplies"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return apierr.JSONInvalidParam(c)
	}
	fields := map[string]any{}
	if req.Notify != nil {
		fields["notify"] = *req.Notify
	}
	if req.WithReplies != nil {
		fields["withReplies"] = *req.WithReplies
	}
	if err := h.followingService.UpdateRelation(me.ID, req.UserID, fields); err != nil {
		return apierr.JSONInternalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}

// UpdateFollowAll handles POST /api/following/update-all.
//
// 呼び出しユーザーの全フォロー関係に対して同じ partial update を適用する。
// 旅行期間中 notify を一括で無効にする等の運用に使う。
func (h *Handler) UpdateFollowAll(c echo.Context) error {
	me := middleware.GetUser(c)
	var req struct {
		Notify      *string `json:"notify"`
		WithReplies *bool   `json:"withReplies"`
	}
	if err := c.Bind(&req); err != nil {
		return apierr.JSONInvalidParam(c)
	}
	fields := map[string]any{}
	if req.Notify != nil {
		fields["notify"] = *req.Notify
	}
	if req.WithReplies != nil {
		fields["withReplies"] = *req.WithReplies
	}
	if len(fields) == 0 {
		return c.NoContent(http.StatusNoContent)
	}
	if err := h.followingService.UpdateAllByFollower(me.ID, fields); err != nil {
		return apierr.JSONInternalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}

// RequestsSent handles POST /api/following/requests/sent.
//
// 呼び出しユーザーが送った未承認の follow request を返す。承認済の関係は
// Following テーブルへ移るのでここには含まれない。
func (h *Handler) RequestsSent(c echo.Context) error {
	me := middleware.GetUser(c)
	var req struct {
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	}
	_ = c.Bind(&req)
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 30
	}
	rows, err := h.followingService.ListSentRequests(me.ID, req.Limit, req.Offset)
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	out := make([]ListRequestsResponseItem, 0, len(rows))
	for _, r := range rows {
		item := ListRequestsResponseItem{ID: r.ID}
		if b, err := h.userService.ShowByID(r.FollowerID); err == nil {
			item.Follower = entity.PackUserDetailed(b.User, b.Profile)
		}
		if b, err := h.userService.ShowByID(r.FolloweeID); err == nil {
			item.Followee = entity.PackUserDetailed(b.User, b.Profile)
		}
		out = append(out, item)
	}
	return c.JSON(http.StatusOK, out)
}
