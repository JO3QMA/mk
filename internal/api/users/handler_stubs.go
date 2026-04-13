package users

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// UserListFavoriteRepository is the interface for user list favorite operations.
type UserListFavoriteRepository interface {
	Create(fav *model.UserListFavorite) error
	Delete(userID, listID string) error
	ListByUser(userID string) ([]*model.UserListFavorite, error)
	Exists(userID, listID string) (bool, error)
}

// SetUserListFavoriteRepo attaches a UserListFavoriteRepository.
func (h *Handler) SetUserListFavoriteRepo(r UserListFavoriteRepository) {
	h.userListFavoriteRepo = r
}

// SetUserListRepo attaches a UserListRepository for list update endpoints.
func (h *Handler) SetUserListRepo(r repository.UserListRepository) {
	h.userListRepo = r
}

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
	user := middleware.GetUser(c)
	var req struct {
		ListID string `json:"listId"`
	}
	if err := c.Bind(&req); err != nil || req.ListID == "" {
		return invalidParam(c)
	}
	if h.userListFavoriteRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	already, _ := h.userListFavoriteRepo.Exists(user.ID, req.ListID)
	if already {
		return c.NoContent(http.StatusNoContent)
	}
	fav := &model.UserListFavorite{
		ID:         h.idGen.Generate(time.Now()),
		UserID:     user.ID,
		UserListID: req.ListID,
	}
	if err := h.userListFavoriteRepo.Create(fav); err != nil {
		return internalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}

// ListsUnfavorite handles POST /api/users/lists/unfavorite.
func (h *Handler) ListsUnfavorite(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		ListID string `json:"listId"`
	}
	if err := c.Bind(&req); err != nil || req.ListID == "" {
		return invalidParam(c)
	}
	if h.userListFavoriteRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	if err := h.userListFavoriteRepo.Delete(user.ID, req.ListID); err != nil {
		return internalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}

// ListsUpdate handles POST /api/users/lists/update.
func (h *Handler) ListsUpdate(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		ListID   string `json:"listId"`
		Name     string `json:"name"`
		IsPublic *bool  `json:"isPublic"`
	}
	if err := c.Bind(&req); err != nil || req.ListID == "" {
		return invalidParam(c)
	}
	if h.userListRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	list, err := h.userListRepo.FindByID(req.ListID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]any{"error": map[string]any{"message": "No such list.", "code": "NO_SUCH_LIST", "id": "796666fe-3dff-4d39-becb-8a5932c1d5b7"}})
	}
	// 所有権チェック
	if list.UserID != user.ID {
		return c.JSON(http.StatusNotFound, map[string]any{"error": map[string]any{"message": "No such list.", "code": "NO_SUCH_LIST", "id": "796666fe-3dff-4d39-becb-8a5932c1d5b7"}})
	}
	fields := map[string]any{}
	if req.Name != "" {
		fields["name"] = req.Name
		list.Name = req.Name
	}
	if req.IsPublic != nil {
		fields["isPublic"] = *req.IsPublic
		list.IsPublic = *req.IsPublic
	}
	if len(fields) > 0 {
		if err := h.userListRepo.UpdateList(req.ListID, fields); err != nil {
			return internalError(c)
		}
	}
	return c.JSON(http.StatusOK, list)
}

// ListsUpdateMembership handles POST /api/users/lists/update-membership.
func (h *Handler) ListsUpdateMembership(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		ListID      string `json:"listId"`
		UserID      string `json:"userId"`
		WithReplies bool   `json:"withReplies"`
	}
	if err := c.Bind(&req); err != nil || req.ListID == "" || req.UserID == "" {
		return invalidParam(c)
	}
	if h.userListRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	list, err := h.userListRepo.FindByID(req.ListID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]any{"error": map[string]any{"message": "No such list.", "code": "NO_SUCH_LIST", "id": "7f44670e-ab16-43b8-b4c1-ccd2ee89cc02"}})
	}
	// 所有権チェック
	if list.UserID != user.ID {
		return c.JSON(http.StatusNotFound, map[string]any{"error": map[string]any{"message": "No such list.", "code": "NO_SUCH_LIST", "id": "7f44670e-ab16-43b8-b4c1-ccd2ee89cc02"}})
	}
	if err := h.userListRepo.UpdateMembership(req.ListID, req.UserID, req.WithReplies); err != nil {
		// メンバーが存在しない場合は404
		return c.JSON(http.StatusNotFound, map[string]any{"error": map[string]any{"message": "No such user.", "code": "NO_SUCH_USER", "id": "588e7f72-c744-4a61-b180-d354e912bda2"}})
	}
	return c.NoContent(http.StatusNoContent)
}

// ListsGetMemberships handles POST /api/users/lists/get-memberships.
func (h *Handler) ListsGetMemberships(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}
