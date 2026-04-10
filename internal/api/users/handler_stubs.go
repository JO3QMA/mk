package users

import (
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/entity"
)

// Achievements handles POST /api/users/achievements.
func (h *Handler) Achievements(c echo.Context) error {
	var req struct {
		UserID string `json:"userId"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return invalidParam(c)
	}
	bundle, err := h.userService.ShowByID(req.UserID)
	if err != nil {
		return noSuchUser(c)
	}
	if bundle.Profile == nil || bundle.Profile.Achievements == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	var achievements []any
	_ = json.Unmarshal(bundle.Profile.Achievements, &achievements)
	return c.JSON(http.StatusOK, achievements)
}

// Clips handles POST /api/users/clips.
func (h *Handler) Clips(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

// Flashs handles POST /api/users/flashs.
func (h *Handler) Flashs(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

// GalleryPosts handles POST /api/users/gallery/posts.
func (h *Handler) GalleryPosts(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

// Pages handles POST /api/users/pages.
func (h *Handler) Pages(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

// GetFrequentlyRepliedUsers handles POST /api/users/get-frequently-replied-users.
func (h *Handler) GetFrequentlyRepliedUsers(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

// GetFollowingUsersByBirthday handles POST /api/users/get-following-users-by-birthday.
func (h *Handler) GetFollowingUsersByBirthday(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

// UserRecommendation handles POST /api/users/recommendation.
func (h *Handler) UserRecommendation(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

// UsersBulk handles POST /api/users — bulk user lookup.
func (h *Handler) UsersBulk(c echo.Context) error {
	var req struct {
		UserIDs []string `json:"userIds"`
	}
	if err := c.Bind(&req); err != nil {
		return invalidParam(c)
	}
	if len(req.UserIDs) == 0 {
		return c.JSON(http.StatusOK, []any{})
	}
	var out []entity.UserLite
	for _, uid := range req.UserIDs {
		if bundle, err := h.userService.ShowByID(uid); err == nil {
			out = append(out, entity.PackUserLite(bundle.User))
		}
	}
	return c.JSON(http.StatusOK, out)
}

// ListsCreateFromPublic handles POST /api/users/lists/create-from-public.
func (h *Handler) ListsCreateFromPublic(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// ListsFavorite handles POST /api/users/lists/favorite.
func (h *Handler) ListsFavorite(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// ListsUnfavorite handles POST /api/users/lists/unfavorite.
func (h *Handler) ListsUnfavorite(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// ListsUpdate handles POST /api/users/lists/update.
func (h *Handler) ListsUpdate(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// ListsUpdateMembership handles POST /api/users/lists/update-membership.
func (h *Handler) ListsUpdateMembership(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// ListsGetMemberships handles POST /api/users/lists/get-memberships.
func (h *Handler) ListsGetMemberships(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}
