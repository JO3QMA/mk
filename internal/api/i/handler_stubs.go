package i

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Apps handles POST /api/i/apps.
func (h *Handler) Apps(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

// AuthorizedApps handles POST /api/i/authorized-apps.
func (h *Handler) AuthorizedApps(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

// SigninHistory handles POST /api/i/signin-history.
func (h *Handler) SigninHistory(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

// RevokeToken handles POST /api/i/revoke-token.
func (h *Handler) RevokeToken(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// UpdateEmail handles POST /api/i/update-email.
func (h *Handler) UpdateEmail(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// Move handles POST /api/i/move.
func (h *Handler) Move(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// GalleryLikes handles POST /api/i/gallery/likes.
func (h *Handler) GalleryLikes(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

// GalleryPosts handles POST /api/i/gallery/posts.
func (h *Handler) GalleryPosts(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

// PageLikes handles POST /api/i/page-likes.
func (h *Handler) PageLikes(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

// RegistryGetDetail handles POST /api/i/registry/get-detail.
func (h *Handler) RegistryGetDetail(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{})
}

// RegistryKeys handles POST /api/i/registry/keys.
func (h *Handler) RegistryKeys(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

// RegistryScopesWithDomain handles POST /api/i/registry/scopes-with-domain.
func (h *Handler) RegistryScopesWithDomain(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}
