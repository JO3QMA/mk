package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

type contextKey string

const UserContextKey contextKey = "misskeyUser"

// AuthMiddleware provides token-based authentication.
type AuthMiddleware struct {
	userRepo        repository.UserRepository
	accessTokenRepo repository.AccessTokenRepository
}

// NewAuthMiddleware creates a new AuthMiddleware.
func NewAuthMiddleware(userRepo repository.UserRepository, accessTokenRepo repository.AccessTokenRepository) *AuthMiddleware {
	return &AuthMiddleware{userRepo: userRepo, accessTokenRepo: accessTokenRepo}
}

// Authenticate is an Echo middleware that extracts and validates the user token.
// It does NOT reject unauthenticated requests - it just sets the user if valid.
func (a *AuthMiddleware) Authenticate() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			token := extractToken(c)
			if token == "" {
				return next(c)
			}

			user, err := a.resolveUser(token)
			if err != nil {
				return next(c)
			}

			c.Set(string(UserContextKey), user)
			return next(c)
		}
	}
}

// RequireAuth is a middleware that requires authentication.
func RequireAuth() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user := GetUser(c)
			if user == nil {
				return c.JSON(http.StatusUnauthorized, map[string]any{
					"error": map[string]any{
						"message": "Authentication is required.",
						"code":    "CREDENTIAL_REQUIRED",
						"id":      "1384574d-a912-4b81-8601-c7b1c4085df1",
					},
				})
			}
			return next(c)
		}
	}
}

// GetUser returns the authenticated user from the context.
func GetUser(c echo.Context) *model.User {
	u, ok := c.Get(string(UserContextKey)).(*model.User)
	if !ok {
		return nil
	}
	return u
}

// RoleChecker abstracts role checking to avoid circular dependency with core/role.
type RoleChecker interface {
	IsAdministrator(userID string) bool
	IsModerator(userID string) bool
}

// RequireAdmin is a middleware that requires the user to have an administrator role.
func RequireAdmin(checker RoleChecker) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user := GetUser(c)
			if user == nil {
				return c.JSON(http.StatusUnauthorized, map[string]any{
					"error": map[string]any{
						"message": "Authentication is required.",
						"code":    "CREDENTIAL_REQUIRED",
						"id":      "1384574d-a912-4b81-8601-c7b1c4085df1",
					},
				})
			}
			if !checker.IsAdministrator(user.ID) {
				return c.JSON(http.StatusForbidden, map[string]any{
					"error": map[string]any{
						"message": "You are not an administrator.",
						"code":    "ROLE_PERMISSION_DENIED",
						"id":      "c3d38592-54c0-429d-bfe8-f1571e00eb14",
					},
				})
			}
			return next(c)
		}
	}
}

// RequireModerator is a middleware that requires the user to have a moderator or admin role.
func RequireModerator(checker RoleChecker) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user := GetUser(c)
			if user == nil {
				return c.JSON(http.StatusUnauthorized, map[string]any{
					"error": map[string]any{
						"message": "Authentication is required.",
						"code":    "CREDENTIAL_REQUIRED",
						"id":      "1384574d-a912-4b81-8601-c7b1c4085df1",
					},
				})
			}
			if !checker.IsModerator(user.ID) {
				return c.JSON(http.StatusForbidden, map[string]any{
					"error": map[string]any{
						"message": "You are not a moderator.",
						"code":    "ROLE_PERMISSION_DENIED",
						"id":      "c3d38592-54c0-429d-bfe8-f1571e00eb14",
					},
				})
			}
			return next(c)
		}
	}
}

func extractToken(c echo.Context) string {
	// Bearer token from Authorization header
	auth := c.Request().Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}

	// Query parameter
	if t := c.QueryParam("i"); t != "" {
		return t
	}

	// JSON body の "i" フィールド (Misskey-style)
	// フロントエンドは全 API で POST {"i":"token", ...} を送信する。
	// ボディを読んだ後にリセットし、後続ハンドラが再読み取り可能にする。
	req := c.Request()
	if req.Body != nil && req.ContentLength != 0 {
		body, err := io.ReadAll(req.Body)
		if err == nil && len(body) > 0 {
			// ボディをリセット
			req.Body = io.NopCloser(bytes.NewReader(body))
			var parsed struct {
				I string `json:"i"`
			}
			if json.Unmarshal(body, &parsed) == nil && parsed.I != "" {
				return parsed.I
			}
		}
	}

	return ""
}

func (a *AuthMiddleware) resolveUser(token string) (*model.User, error) {
	// まずnative tokenで検索
	user, err := a.userRepo.FindByToken(token)
	if err == nil {
		return user, nil
	}

	// access tokenのhashで検索
	hash := sha256Hash(token)
	accessToken, err := a.accessTokenRepo.FindByHash(hash)
	if err != nil {
		return nil, err
	}

	return accessToken.User, nil
}

func sha256Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
