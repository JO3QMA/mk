package following

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// RequestsSent handles POST /api/following/requests/sent.
//
// 呼び出しユーザーが送った未承認の follow request を返す。承認済の関係は
// Following テーブルへ移るのでここには含まれない。
func (h *Handler) RequestsSent(c echo.Context) error {
	me := middleware.GetUser(c)
	var req struct {
		Limit   int    `json:"limit"`
		SinceID string `json:"sinceId"`
		UntilID string `json:"untilId"`
	}
	_ = c.Bind(&req)
	rows, err := h.followingService.ListSentRequests(me.ID, req.Limit, req.SinceID, req.UntilID)
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
