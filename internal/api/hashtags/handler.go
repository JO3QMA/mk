package hashtags

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// Handler handles hashtag-related API endpoints.
type Handler struct {
	db *gorm.DB
}

// NewHandler creates a new hashtags Handler.
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{db: db}
}

// List handles POST /api/hashtags/list.
func (h *Handler) List(c echo.Context) error {
	var req struct {
		Limit  int    `json:"limit"`
		Sort   string `json:"sort"`
		Offset int    `json:"offset"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "Invalid parameters.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Limit > 100 {
		req.Limit = 100
	}
	q := h.db.Model(&model.Hashtag{}).Order("\"mentionedUsersCount\" DESC").Limit(req.Limit)
	if req.Offset > 0 {
		q = q.Offset(req.Offset)
	}
	var tags []*model.Hashtag
	if err := q.Find(&tags).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	result := make([]map[string]any, 0, len(tags))
	for _, t := range tags {
		result = append(result, packTag(t))
	}
	return c.JSON(http.StatusOK, result)
}

// Search handles POST /api/hashtags/search.
func (h *Handler) Search(c echo.Context) error {
	var req struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := c.Bind(&req); err != nil || req.Query == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "query is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}
	var tags []*model.Hashtag
	if err := h.db.Where("name ILIKE ?", "%"+req.Query+"%").Limit(req.Limit).Find(&tags).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	names := make([]string, 0, len(tags))
	for _, t := range tags {
		names = append(names, t.Name)
	}
	return c.JSON(http.StatusOK, names)
}

// Show handles POST /api/hashtags/show.
func (h *Handler) Show(c echo.Context) error {
	var req struct {
		Tag string `json:"tag"`
	}
	if err := c.Bind(&req); err != nil || req.Tag == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "tag is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	var tag model.Hashtag
	if err := h.db.Where("name = ?", req.Tag).First(&tag).Error; err != nil {
		return c.JSON(http.StatusNotFound, errResp("NO_SUCH_HASHTAG", "No such hashtag.", "110ee688-193e-4a3a-9ecf-c167234e6f7d"))
	}
	return c.JSON(http.StatusOK, packTag(&tag))
}

// Trend handles POST /api/hashtags/trend.
func (h *Handler) Trend(c echo.Context) error {
	var tags []*model.Hashtag
	if err := h.db.Order("\"mentionedUsersCount\" DESC").Limit(10).Find(&tags).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	result := make([]map[string]any, 0, len(tags))
	for _, t := range tags {
		result = append(result, map[string]any{
			"tag":        t.Name,
			"chart":      []int{},
			"usersCount": t.MentionedUsersCount,
		})
	}
	return c.JSON(http.StatusOK, result)
}

// Users handles POST /api/hashtags/users.
func (h *Handler) Users(c echo.Context) error {
	var req struct {
		Tag   string `json:"tag"`
		Limit int    `json:"limit"`
	}
	if err := c.Bind(&req); err != nil || req.Tag == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "tag is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	// ハッシュタグを使ったユーザー一覧 (簡易版: 空配列)
	return c.JSON(http.StatusOK, []any{})
}

func packTag(t *model.Hashtag) map[string]any {
	return map[string]any{
		"tag":                       t.Name,
		"mentionedUsersCount":       t.MentionedUsersCount,
		"mentionedLocalUsersCount":  t.MentionedLocalUsersCount,
		"mentionedRemoteUsersCount": t.MentionedRemoteUsersCount,
		"attachedUsersCount":        t.AttachedUsersCount,
		"attachedLocalUsersCount":   t.AttachedLocalUsersCount,
		"attachedRemoteUsersCount":  t.AttachedRemoteUsersCount,
	}
}

func errResp(code, message, id string) map[string]any {
	return map[string]any{
		"error": map[string]any{"message": message, "code": code, "id": id},
	}
}
