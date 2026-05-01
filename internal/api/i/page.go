package i

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// PageLikes handles POST /api/i/page-likes.
// 自分が like した page 一覧を返却する。
func (h *Handler) PageLikes(c echo.Context) error {
	if h.pageLikeRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	u := middleware.GetUser(c)
	limit, offset, _, _ := paginationFromRequest(c)
	likes, err := h.pageLikeRepo.ListByUser(u.ID, limit, offset)
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	out := make([]map[string]any, 0, len(likes))
	for _, l := range likes {
		out = append(out, map[string]any{
			"id":     l.ID,
			"pageId": l.PageID,
		})
	}
	return c.JSON(http.StatusOK, out)
}
