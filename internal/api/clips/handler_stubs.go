package clips

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Favorite handles POST /api/clips/favorite.
func (h *Handler) Favorite(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// Unfavorite handles POST /api/clips/unfavorite.
func (h *Handler) Unfavorite(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// MyFavorites handles POST /api/clips/my-favorites.
func (h *Handler) MyFavorites(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}
