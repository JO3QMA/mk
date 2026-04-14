package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	"github.com/shiroha-a/mk/internal/config"
	"github.com/shiroha-a/mk/internal/misc"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// Handler handles MiAuth endpoints.
type Handler struct {
	repo  repository.AuthSessionRepository
	cfg   *config.Config
	idGen id.Generator
}

// NewHandler creates a new auth handler.
func NewHandler(repo repository.AuthSessionRepository, cfg *config.Config, idGen id.Generator) *Handler {
	return &Handler{repo: repo, cfg: cfg, idGen: idGen}
}

func apiError(code, message, errID string) map[string]any {
	return apierr.Error(code, message, errID)
}

// SessionGenerate handles POST /api/auth/session/generate.
func (h *Handler) SessionGenerate(c echo.Context) error {
	var req struct {
		AppSecret string `json:"appSecret"`
	}
	if err := c.Bind(&req); err != nil || req.AppSecret == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "appSecret is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}

	app, err := h.repo.FindAppBySecret(req.AppSecret)
	if err != nil {
		return c.JSON(http.StatusNotFound, apiError("NO_SUCH_APP", "No such app.", "c5628b5a-3c9e-4e3f-b765-44952b2bfb0e"))
	}

	token := uuid.New().String()
	now := time.Now()
	session := &model.AuthSession{
		ID:        h.idGen.Generate(now),
		CreatedAt: now,
		Token:     token,
		AppID:     app.ID,
	}
	if err := h.repo.CreateSession(session); err != nil {
		return c.JSON(http.StatusInternalServerError, apiError("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}

	return c.JSON(http.StatusOK, map[string]any{
		"token": token,
		"url":   fmt.Sprintf("%s/auth/%s", h.cfg.URL, token),
	})
}

// SessionShow handles POST /api/auth/session/show.
func (h *Handler) SessionShow(c echo.Context) error {
	var req struct {
		Token string `json:"token"`
	}
	if err := c.Bind(&req); err != nil || req.Token == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "token is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}

	session, err := h.repo.FindSessionByToken(req.Token)
	if err != nil {
		return c.JSON(http.StatusNotFound, apiError("NO_SUCH_SESSION", "No such session.", "bd72c97d-eca4-4403-8269-7b9cc0b9a2c0"))
	}

	return c.JSON(http.StatusOK, packSession(session))
}

// Accept handles POST /api/auth/accept.
func (h *Handler) Accept(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		Token string `json:"token"`
	}
	if err := c.Bind(&req); err != nil || req.Token == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "token is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}

	session, err := h.repo.FindSessionByToken(req.Token)
	if err != nil {
		return c.JSON(http.StatusNotFound, apiError("NO_SUCH_SESSION", "No such session.", "bd72c97d-eca4-4403-8269-7b9cc0b9a2c0"))
	}

	// アクセストークンが既に存在するか確認
	_, tokenErr := h.repo.FindAccessTokenByAppAndUser(session.AppID, user.ID)
	if tokenErr != nil {
		// アクセストークンを新規生成
		tokenStr := misc.SecureRandomHex(32)
		hash := sha256Hex(tokenStr + session.App.Secret)
		now := time.Now()
		accessToken := &model.AccessToken{
			ID:         h.idGen.Generate(now),
			Token:      tokenStr,
			Hash:       hash,
			UserID:     user.ID,
			AppID:      &session.AppID,
			Permission: session.App.Permission,
			LastUsedAt: &now,
		}
		if err := h.repo.CreateAccessToken(accessToken); err != nil {
			return c.JSON(http.StatusInternalServerError, apiError("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
		}
	}

	// セッションにユーザーIDを設定
	if err := h.repo.UpdateSessionUserID(session.ID, user.ID); err != nil {
		return c.JSON(http.StatusInternalServerError, apiError("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}

	return c.NoContent(http.StatusNoContent)
}

// SessionUserkey handles POST /api/auth/session/userkey.
func (h *Handler) SessionUserkey(c echo.Context) error {
	var req struct {
		AppSecret string `json:"appSecret"`
		Token     string `json:"token"`
	}
	if err := c.Bind(&req); err != nil || req.AppSecret == "" || req.Token == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "appSecret and token are required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}

	app, err := h.repo.FindAppBySecret(req.AppSecret)
	if err != nil {
		return c.JSON(http.StatusNotFound, apiError("NO_SUCH_APP", "No such app.", "c5628b5a-3c9e-4e3f-b765-44952b2bfb0e"))
	}

	session, err := h.repo.FindSessionByTokenAndAppID(req.Token, app.ID)
	if err != nil {
		return c.JSON(http.StatusNotFound, apiError("NO_SUCH_SESSION", "No such session.", "bd72c97d-eca4-4403-8269-7b9cc0b9a2c0"))
	}

	if session.UserID == nil {
		return c.JSON(http.StatusForbidden, apiError("PENDING_SESSION", "This session is not yet approved.", "8c8a4145-02cc-4cca-8e66-29ba60445a8e"))
	}

	accessToken, err := h.repo.FindAccessTokenByAppAndUser(app.ID, *session.UserID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apiError("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}

	// セッション削除（使い捨て）
	_ = h.repo.DeleteSession(session.ID)

	resp := map[string]any{
		"accessToken": accessToken.Token,
		"user":        packUser(session.User),
	}
	return c.JSON(http.StatusOK, resp)
}

// GenToken handles POST /api/miauth/gen-token.
// MiAuth用のアクセストークンを直接生成する（App不要）。
func (h *Handler) GenToken(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		Session     *string  `json:"session"`
		Name        *string  `json:"name"`
		Description *string  `json:"description"`
		IconURL     *string  `json:"iconUrl"`
		Permission  []string `json:"permission"`
	}
	if err := c.Bind(&req); err != nil || req.Permission == nil {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "permission is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}

	tokenStr := misc.SecureRandomHex(32)
	now := time.Now()
	accessToken := &model.AccessToken{
		ID:          h.idGen.Generate(now),
		LastUsedAt:  &now,
		Token:       tokenStr,
		Hash:        sha256Hex(tokenStr),
		UserID:      user.ID,
		Session:     req.Session,
		Name:        req.Name,
		Description: req.Description,
		IconURL:     req.IconURL,
		Permission:  req.Permission,
	}
	if err := h.repo.CreateAccessToken(accessToken); err != nil {
		return c.JSON(http.StatusInternalServerError, apiError("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}

	return c.JSON(http.StatusOK, map[string]any{"token": tokenStr})
}

func packSession(s *model.AuthSession) map[string]any {
	result := map[string]any{
		"id":    s.ID,
		"token": s.Token,
	}
	if s.App != nil {
		result["app"] = map[string]any{
			"id":          s.App.ID,
			"name":        s.App.Name,
			"callbackUrl": s.App.CallbackURL,
			"permission":  s.App.Permission,
		}
	}
	return result
}

func packUser(u *model.User) map[string]any {
	if u == nil {
		return map[string]any{}
	}
	return map[string]any{
		"id":       u.ID,
		"username": u.Username,
		"name":     u.Name,
		"host":     u.Host,
	}
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
