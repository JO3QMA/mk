package notes

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// TimelineRequest is the common request body for the four timeline endpoints.
type TimelineRequest struct {
	Limit   int    `json:"limit"`
	SinceID string `json:"sinceId"`
	UntilID string `json:"untilId"`
}

func (r *TimelineRequest) normalize() {
	if r.Limit <= 0 {
		r.Limit = 10
	}
	if r.Limit > 100 {
		r.Limit = 100
	}
}

// Timeline handles POST /api/notes/timeline (home timeline).
func (h *Handler) Timeline(c echo.Context) error {
	return h.serveTimeline(c, func(viewer *model.User, req TimelineRequest) ([]*model.Note, error) {
		return h.timelineService.HomeTimeline(c.Request().Context(), viewer, req.UntilID, req.SinceID, req.Limit)
	}, true)
}

// LocalTimeline handles POST /api/notes/local-timeline.
func (h *Handler) LocalTimeline(c echo.Context) error {
	return h.serveTimeline(c, func(viewer *model.User, req TimelineRequest) ([]*model.Note, error) {
		return h.timelineService.LocalTimeline(c.Request().Context(), viewer, req.UntilID, req.SinceID, req.Limit)
	}, false)
}

// GlobalTimeline handles POST /api/notes/global-timeline.
func (h *Handler) GlobalTimeline(c echo.Context) error {
	return h.serveTimeline(c, func(viewer *model.User, req TimelineRequest) ([]*model.Note, error) {
		return h.timelineService.GlobalTimeline(c.Request().Context(), viewer, req.UntilID, req.SinceID, req.Limit)
	}, false)
}

// HybridTimeline handles POST /api/notes/hybrid-timeline.
func (h *Handler) HybridTimeline(c echo.Context) error {
	return h.serveTimeline(c, func(viewer *model.User, req TimelineRequest) ([]*model.Note, error) {
		return h.timelineService.HybridTimeline(c.Request().Context(), viewer, req.UntilID, req.SinceID, req.Limit)
	}, true)
}

// serveTimeline factors out the common parsing and error handling for the four
// timeline endpoints. requireAuthがtrueのときviewer==nilで401相当を返す。
func (h *Handler) serveTimeline(
	c echo.Context,
	fn func(*model.User, TimelineRequest) ([]*model.Note, error),
	requireAuth bool,
) error {
	var req TimelineRequest
	if err := c.Bind(&req); err != nil {
		return invalidParam(c)
	}
	req.normalize()

	viewer := middleware.GetUser(c)
	if requireAuth && viewer == nil {
		return c.JSON(http.StatusUnauthorized, map[string]any{
			"error": map[string]any{
				"message": "Credential required.",
				"code":    "CREDENTIAL_REQUIRED",
				"id":      "1384574d-a912-4b81-8601-c7b1c4085df1",
			},
		})
	}

	// UGC visibility: 未ログインユーザーの閲覧を制限する (meta.ugcVisibilityForVisitor)。
	// "none" → 空リスト、"local" → local timeline のみ許可 (global はブロック)。
	if viewer == nil && h.ugcVisibility == "none" {
		return c.JSON(http.StatusOK, []any{})
	}

	notes, err := fn(viewer, req)
	if err != nil {
		// requireAuthでviewer nilチェックは事前に行っているので、Service層からの
		// ErrUnauthenticatedはここには到達しない。残りはRedis等の障害のみ。
		return internalError(c)
	}
	return c.JSON(http.StatusOK, h.packMany(notes, viewer))
}
