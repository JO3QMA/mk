package app

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// Handler handles app-related API endpoints.
type Handler struct {
	repo  repository.AuthSessionRepository
	idGen id.Generator
}

// NewHandler creates a new app Handler.
func NewHandler(repo repository.AuthSessionRepository, idGen id.Generator) *Handler {
	return &Handler{repo: repo, idGen: idGen}
}

func apiError(code, message, errID string) map[string]any {
	return map[string]any{
		"error": map[string]any{"message": message, "code": code, "id": errID},
	}
}

// Create handles POST /api/app/create.
func (h *Handler) Create(c echo.Context) error {
	var req struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Permission  []string `json:"permission"`
		CallbackURL *string  `json:"callbackUrl"`
	}
	if err := c.Bind(&req); err != nil || req.Name == "" || req.Description == "" || req.Permission == nil {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "name, description, and permission are required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}

	// 認証済みユーザーのIDを取得（任意）
	var userID *string
	if u := middleware.GetUser(c); u != nil {
		userID = &u.ID
	}

	now := time.Now()
	a := &model.App{
		ID:          h.idGen.Generate(now),
		CreatedAt:   now,
		UserID:      userID,
		Secret:      secureRandomHex(32),
		Name:        req.Name,
		Description: req.Description,
		Permission:  pq.StringArray(req.Permission),
		CallbackURL: req.CallbackURL,
	}
	if err := h.repo.CreateApp(a); err != nil {
		return c.JSON(http.StatusInternalServerError, apiError("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}

	return c.JSON(http.StatusOK, packApp(a, true))
}

// Show handles POST /api/app/show.
func (h *Handler) Show(c echo.Context) error {
	var req struct {
		AppID string `json:"appId"`
	}
	if err := c.Bind(&req); err != nil || req.AppID == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "appId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}

	a, err := h.repo.FindAppByID(req.AppID)
	if err != nil {
		return c.JSON(http.StatusNotFound, apiError("NO_SUCH_APP", "No such app.", "dce83913-2dc6-4093-8a7b-71dbb11718a3"))
	}

	// 認証済み && 自分のアプリならsecretも返す
	includeSecret := false
	if u := middleware.GetUser(c); u != nil && a.UserID != nil && *a.UserID == u.ID {
		includeSecret = true
	}

	return c.JSON(http.StatusOK, packApp(a, includeSecret))
}

// MyApps handles POST /api/my/apps.
func (h *Handler) MyApps(c echo.Context) error {
	u := middleware.GetUser(c)
	var req struct {
		Limit  *int `json:"limit"`
		Offset *int `json:"offset"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "Invalid parameters.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}

	limit := 10
	if req.Limit != nil {
		limit = *req.Limit
		if limit < 1 {
			limit = 1
		}
		if limit > 100 {
			limit = 100
		}
	}
	offset := 0
	if req.Offset != nil && *req.Offset > 0 {
		offset = *req.Offset
	}

	apps, err := h.repo.ListAppsByUserID(u.ID, limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apiError("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}

	result := make([]map[string]any, len(apps))
	for i, a := range apps {
		result[i] = packApp(a, true)
	}
	return c.JSON(http.StatusOK, result)
}

func packApp(a *model.App, includeSecret bool) map[string]any {
	resp := map[string]any{
		"id":           a.ID,
		"name":         a.Name,
		"callbackUrl":  a.CallbackURL,
		"permission":   a.Permission,
		"isAuthorized": false,
	}
	if includeSecret {
		resp["secret"] = a.Secret
	}
	return resp
}

func secureRandomHex(n int) string {
	b := make([]byte, (n+1)/2)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)[:n]
}
