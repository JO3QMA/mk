package signin

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

// Handler handles signin-related API endpoints.
type Handler struct {
	userRepo repository.UserRepository
}

// NewHandler creates a new signin Handler.
func NewHandler(userRepo repository.UserRepository) *Handler {
	return &Handler{userRepo: userRepo}
}

// Signin handles POST /api/signin.
// Misskey フロントエンドのログインフロー:
// 1. username のみ → { finished: false, next: "password" or "captcha" }
// 2. username + password → { finished: true, id: "...", i: "token" }
func (h *Handler) Signin(c echo.Context) error {
	var req struct {
		Username string  `json:"username"`
		Password *string `json:"password"`
	}
	if err := c.Bind(&req); err != nil || req.Username == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error": map[string]any{
				"id": "6cc579cc-885d-43d8-95c2-b8c7fc963280",
			},
		})
	}

	// ユーザー検索
	user, err := h.userRepo.FindByUsernameLower(req.Username, nil)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]any{
			"error": map[string]any{
				"id": "6cc579cc-885d-43d8-95c2-b8c7fc963280",
			},
		})
	}

	if user.IsSuspended {
		return c.JSON(http.StatusForbidden, map[string]any{
			"error": map[string]any{
				"id": "e03a5f46-d309-4865-9b69-56282d94e1eb",
			},
		})
	}

	// Step 1: パスワードなしの場合、次のステップを返す
	if req.Password == nil {
		return c.JSON(http.StatusOK, map[string]any{
			"finished": false,
			"next":     "password",
		})
	}

	// Step 2: パスワード検証
	profile, err := h.userRepo.FindProfileByUserID(user.ID)
	if err != nil || profile.Password == nil {
		return c.JSON(http.StatusForbidden, map[string]any{
			"error": map[string]any{
				"id": "932c904e-9460-45b7-9ce6-7ed33be7eb2c",
			},
		})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*profile.Password), []byte(*req.Password)); err != nil {
		return c.JSON(http.StatusForbidden, map[string]any{
			"error": map[string]any{
				"id": "932c904e-9460-45b7-9ce6-7ed33be7eb2c",
			},
		})
	}

	// 認証成功
	token := ""
	if user.Token != nil {
		token = *user.Token
	}
	return c.JSON(http.StatusOK, map[string]any{
		"finished": true,
		"id":       user.ID,
		"i":        token,
	})
}
