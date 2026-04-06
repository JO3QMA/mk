package users

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/misskey-dev/misskey-go/internal/entity"
	"github.com/misskey-dev/misskey-go/internal/repository"
)

// Handler handles user-related API endpoints.
type Handler struct {
	userRepo repository.UserRepository
}

// NewHandler creates a new users Handler.
func NewHandler(userRepo repository.UserRepository) *Handler {
	return &Handler{userRepo: userRepo}
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

	if req.UserID == nil && req.Username == nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error": map[string]any{
				"message": "userId or username is required.",
				"code":    "INVALID_PARAM",
				"id":      "3d81ceae-475f-4600-b2a8-2bc116157532",
			},
		})
	}

	var userID string
	if req.UserID != nil {
		user, err := h.userRepo.FindByID(*req.UserID)
		if err != nil {
			return noSuchUser(c)
		}
		userID = user.ID
	} else {
		user, err := h.userRepo.FindByUsernameLower(*req.Username, req.Host)
		if err != nil {
			return noSuchUser(c)
		}
		userID = user.ID
	}

	user, err := h.userRepo.FindByID(userID)
	if err != nil {
		return noSuchUser(c)
	}

	profile, _ := h.userRepo.FindProfileByUserID(user.ID)

	return c.JSON(http.StatusOK, entity.PackUserDetailed(user, profile))
}

func noSuchUser(c echo.Context) error {
	return c.JSON(http.StatusNotFound, map[string]any{
		"error": map[string]any{
			"message": "No such user.",
			"code":    "NO_SUCH_USER",
			"id":      "4362f8dc-731f-4ad8-a694-be5a88922a24",
		},
	})
}
