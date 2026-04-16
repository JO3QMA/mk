package users

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/shiroha-a/mk/internal/server/middleware"
	"gorm.io/gorm"
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
		return apierr.JSONInvalidParam(c)
	}
	bundle, err := h.userService.ShowByID(req.UserID)
	if err != nil {
		return apierr.JSONNoSuchUser(c)
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
	var req struct {
		UserID string `json:"userId"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return apierr.JSONInvalidParam(c)
	}
	if h.clipRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	clampListLimit(&req.Limit)
	rows, err := h.clipRepo.ListByUser(req.UserID, req.Limit, req.Offset)
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	// 他人のプロフィールから見えるのは public のみ (本家 Misskey 同仕様)。
	viewer := middleware.GetUser(c)
	isSelf := viewer != nil && viewer.ID == req.UserID
	out := make([]map[string]any, 0, len(rows))
	for _, cl := range rows {
		if !isSelf && !cl.IsPublic {
			continue
		}
		out = append(out, map[string]any{
			"id":            cl.ID,
			"userId":        cl.UserID,
			"name":          cl.Name,
			"description":   cl.Description,
			"isPublic":      cl.IsPublic,
			"notesCount":    cl.NotesCount,
			"lastClippedAt": cl.LastClippedAt,
		})
	}
	return c.JSON(http.StatusOK, out)
}

// Flashs handles POST /api/users/flashs.
func (h *Handler) Flashs(c echo.Context) error {
	var req struct {
		UserID string `json:"userId"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return apierr.JSONInvalidParam(c)
	}
	if h.flashRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	clampListLimit(&req.Limit)
	rows, err := h.flashRepo.ListByUser(req.UserID, req.Limit, req.Offset)
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	viewer := middleware.GetUser(c)
	isSelf := viewer != nil && viewer.ID == req.UserID
	out := make([]map[string]any, 0, len(rows))
	for _, f := range rows {
		if !isSelf && f.Visibility != "public" {
			continue
		}
		out = append(out, map[string]any{
			"id":          f.ID,
			"updatedAt":   f.UpdatedAt,
			"title":       f.Title,
			"summary":     f.Summary,
			"userId":      f.UserID,
			"script":      f.Script,
			"permissions": []string(f.Permissions),
			"likedCount":  f.LikedCount,
			"visibility":  f.Visibility,
		})
	}
	return c.JSON(http.StatusOK, out)
}

// GalleryPosts handles POST /api/users/gallery/posts.
func (h *Handler) GalleryPosts(c echo.Context) error {
	var req struct {
		UserID string `json:"userId"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return apierr.JSONInvalidParam(c)
	}
	if h.galleryRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	clampListLimit(&req.Limit)
	rows, err := h.galleryRepo.ListByUser(req.UserID, req.Limit, req.Offset)
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	// GalleryPost に visibility 概念はなく常に公開扱い。
	out := make([]map[string]any, 0, len(rows))
	for _, g := range rows {
		out = append(out, map[string]any{
			"id":          g.ID,
			"updatedAt":   g.UpdatedAt,
			"userId":      g.UserID,
			"title":       g.Title,
			"description": g.Description,
			"fileIds":     []string(g.FileIDs),
			"tags":        []string(g.Tags),
			"isSensitive": g.IsSensitive,
			"likedCount":  g.LikedCount,
			"files":       []any{},
		})
	}
	return c.JSON(http.StatusOK, out)
}

// Pages handles POST /api/users/pages.
func (h *Handler) Pages(c echo.Context) error {
	var req struct {
		UserID string `json:"userId"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return apierr.JSONInvalidParam(c)
	}
	if h.pageRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	clampListLimit(&req.Limit)
	rows, err := h.pageRepo.ListByUser(req.UserID, req.Limit, req.Offset)
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	viewer := middleware.GetUser(c)
	isSelf := viewer != nil && viewer.ID == req.UserID
	out := make([]map[string]any, 0, len(rows))
	for _, p := range rows {
		if !isSelf && p.Visibility != model.PageVisibilityPublic {
			continue
		}
		out = append(out, map[string]any{
			"id":        p.ID,
			"updatedAt": p.UpdatedAt,
			"userId":    p.UserID,
			"title":     p.Title,
			"name":      p.Name,
			"summary":   p.Summary,
		})
	}
	return c.JSON(http.StatusOK, out)
}

// clampListLimit normalises a user-supplied limit to 1..100 with a default of 10.
func clampListLimit(limit *int) {
	if *limit <= 0 {
		*limit = 10
	}
	if *limit > 100 {
		*limit = 100
	}
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
//
// Misskey 本家は userIds を最大 100 件に制限している。未知 ID は無視される
// (他の ID の結果は返す)。
func (h *Handler) UsersBulk(c echo.Context) error {
	var req struct {
		UserIDs []string `json:"userIds"`
	}
	if err := c.Bind(&req); err != nil {
		return apierr.JSONInvalidParam(c)
	}
	if len(req.UserIDs) == 0 {
		return c.JSON(http.StatusOK, []any{})
	}
	if len(req.UserIDs) > 100 {
		req.UserIDs = req.UserIDs[:100]
	}
	out := make([]entity.UserLite, 0, len(req.UserIDs))
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
		return apierr.JSONInvalidParam(c)
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
		return apierr.JSONInternalError(c)
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
		return apierr.JSONInvalidParam(c)
	}
	if h.userListFavoriteRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	if err := h.userListFavoriteRepo.Delete(user.ID, req.ListID); err != nil {
		return apierr.JSONInternalError(c)
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
		return apierr.JSONInvalidParam(c)
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
			return apierr.JSONInternalError(c)
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
		return apierr.JSONInvalidParam(c)
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.JSON(http.StatusNotFound, map[string]any{"error": map[string]any{"message": "No such user.", "code": "NO_SUCH_USER", "id": "588e7f72-c744-4a61-b180-d354e912bda2"}})
		}
		return apierr.JSONInternalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}

// ListsGetMemberships handles POST /api/users/lists/get-memberships.
//
// 認証ユーザが所有する UserList のうち、指定された userId を member として含む
// ものの一覧を返す。Misskey 本家互換。
func (h *Handler) ListsGetMemberships(c echo.Context) error {
	viewer := middleware.GetUser(c)
	if viewer == nil {
		return apierr.JSONAccessDenied(c)
	}
	var req struct {
		UserID string `json:"userId"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return apierr.JSONInvalidParam(c)
	}
	if h.userListRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	lists, err := h.userListRepo.ListsContainingMember(viewer.ID, req.UserID)
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	out := make([]map[string]any, 0, len(lists))
	for _, l := range lists {
		out = append(out, map[string]any{
			"id":       l.ID,
			"name":     l.Name,
			"userId":   l.UserID,
			"isPublic": l.IsPublic,
		})
	}
	return c.JSON(http.StatusOK, out)
}
