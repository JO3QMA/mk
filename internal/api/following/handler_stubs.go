package following

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Invalidate handles POST /api/following/invalidate.
func (h *Handler) Invalidate(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// UpdateFollow handles POST /api/following/update.
func (h *Handler) UpdateFollow(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// UpdateFollowAll handles POST /api/following/update-all.
func (h *Handler) UpdateFollowAll(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// RequestsSent handles POST /api/following/requests/sent.
func (h *Handler) RequestsSent(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}
