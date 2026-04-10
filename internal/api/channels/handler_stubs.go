package channels

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Favorite handles POST /api/channels/favorite.
func (h *Handler) Favorite(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// Unfavorite handles POST /api/channels/unfavorite.
func (h *Handler) Unfavorite(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// MuteCreate handles POST /api/channels/mute/create.
func (h *Handler) MuteCreate(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// MuteDelete handles POST /api/channels/mute/delete.
func (h *Handler) MuteDelete(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// MyFavorites handles POST /api/channels/my-favorites.
func (h *Handler) MyFavorites(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

// MuteList handles POST /api/channels/mute/list.
func (h *Handler) MuteList(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}
