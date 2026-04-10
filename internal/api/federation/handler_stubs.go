package federation

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Followers handles POST /api/federation/followers.
func (h *Handler) Followers(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

// Following handles POST /api/federation/following.
func (h *Handler) Following(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

// Users handles POST /api/federation/users.
func (h *Handler) Users(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

// Stats handles POST /api/federation/stats.
func (h *Handler) Stats(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{
		"topSubInstances": []any{}, "otherFollowersCount": 0,
		"topPubInstances": []any{}, "otherFollowingCount": 0,
	})
}

// UpdateRemoteUser handles POST /api/federation/update-remote-user.
func (h *Handler) UpdateRemoteUser(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}
