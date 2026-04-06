package users

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/misskey-dev/misskey-go/internal/entity"
	"github.com/misskey-dev/misskey-go/internal/model"
	"gorm.io/gorm"
)

// Handler handles user-related API endpoints.
type Handler struct {
	db *gorm.DB
}

// NewHandler creates a new users Handler.
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{db: db}
}

// ShowRequest is the request body for users/show.
type ShowRequest struct {
	UserID   *string `json:"userId"`
	Username *string `json:"username"`
	Host     *string `json:"host"`
}

// Show handles POST /api/users/show.
func (h *Handler) Show(c echo.Context) error {
	var req ShowRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error": map[string]any{
				"message": "Invalid param.",
				"code":    "INVALID_PARAM",
				"id":      "3d81ceae-475f-4600-b2a8-2bc116157532",
			},
		})
	}

	var user model.User
	var err error

	if req.UserID != nil {
		err = h.db.First(&user, "id = ?", *req.UserID).Error
	} else if req.Username != nil {
		q := h.db.Where("\"usernameLower\" = lower(?)", *req.Username)
		if req.Host != nil {
			q = q.Where("host = ?", *req.Host)
		} else {
			q = q.Where("host IS NULL")
		}
		err = q.First(&user).Error
	} else {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error": map[string]any{
				"message": "userId or username is required.",
				"code":    "INVALID_PARAM",
				"id":      "3d81ceae-475f-4600-b2a8-2bc116157532",
			},
		})
	}

	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]any{
			"error": map[string]any{
				"message": "No such user.",
				"code":    "NO_SUCH_USER",
				"id":      "4362f8dc-731f-4ad8-a694-be5a88922a24",
			},
		})
	}

	// UserProfileを取得
	var profile model.UserProfile
	h.db.First(&profile, "\"userId\" = ?", user.ID)

	return c.JSON(http.StatusOK, entity.PackUserDetailed(&user, &profile))
}
