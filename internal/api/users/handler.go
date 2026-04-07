package users

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/core/user"
	"github.com/shiroha-a/mk/internal/entity"
)

// Handler handles user-related API endpoints.
type Handler struct {
	userService *user.Service
}

// NewHandler creates a new users Handler.
func NewHandler(userService *user.Service) *Handler {
	return &Handler{userService: userService}
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
		return invalidParam(c)
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

	var (
		bundle *user.UserWithProfile
		err    error
	)
	if req.UserID != nil {
		bundle, err = h.userService.ShowByID(*req.UserID)
	} else {
		bundle, err = h.userService.ShowByUsername(*req.Username, req.Host)
	}

	if err != nil {
		// Service.ShowByID/ShowByUsernameはErrUserNotFoundのみ返す
		return noSuchUser(c)
	}

	return c.JSON(http.StatusOK, entity.PackUserDetailed(bundle.User, bundle.Profile))
}

func invalidParam(c echo.Context) error {
	return c.JSON(http.StatusBadRequest, map[string]any{
		"error": map[string]any{
			"message": "Invalid param.",
			"code":    "INVALID_PARAM",
			"id":      "3d81ceae-475f-4600-b2a8-2bc116157532",
		},
	})
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
