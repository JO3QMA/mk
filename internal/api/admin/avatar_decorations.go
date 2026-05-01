package admin

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	"github.com/shiroha-a/mk/internal/model"
)

// AvatarDecorationsCreate handles POST /api/admin/avatar-decorations/create.
func (h *Handler) AvatarDecorationsCreate(c echo.Context) error {
	if h.avatarDecoRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		URL         string   `json:"url"`
		RoleIDs     []string `json:"roleIdsThatCanBeUsedThisDecoration"`
	}
	if err := c.Bind(&req); err != nil || req.Name == "" || req.URL == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "Invalid parameters.", "00000000-0000-0000-0000-000000000000"))
	}
	d := &model.AvatarDecoration{
		ID:          h.idGen.Generate(time.Now()),
		Name:        req.Name,
		Description: req.Description,
		URL:         req.URL,
		RoleIDs:     req.RoleIDs,
	}
	if err := h.avatarDecoRepo.Create(d); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	return c.JSON(http.StatusOK, d)
}

// AvatarDecorationsDelete handles POST /api/admin/avatar-decorations/delete.
func (h *Handler) AvatarDecorationsDelete(c echo.Context) error {
	if h.avatarDecoRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		ID string `json:"id"`
	}
	_ = c.Bind(&req)
	_ = h.avatarDecoRepo.Delete(req.ID)
	return c.NoContent(http.StatusNoContent)
}

// AvatarDecorationsList handles POST /api/admin/avatar-decorations/list.
func (h *Handler) AvatarDecorationsList(c echo.Context) error {
	if h.avatarDecoRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	rows, err := h.avatarDecoRepo.List()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	if rows == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	return c.JSON(http.StatusOK, rows)
}

// AvatarDecorationsUpdate handles POST /api/admin/avatar-decorations/update.
func (h *Handler) AvatarDecorationsUpdate(c echo.Context) error {
	if h.avatarDecoRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		ID          string    `json:"id"`
		Name        *string   `json:"name"`
		Description *string   `json:"description"`
		URL         *string   `json:"url"`
		RoleIDs     *[]string `json:"roleIdsThatCanBeUsedThisDecoration"`
	}
	if err := c.Bind(&req); err != nil || req.ID == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "Invalid parameters.", "00000000-0000-0000-0000-000000000000"))
	}
	if _, err := h.avatarDecoRepo.FindByID(req.ID); err != nil {
		return c.JSON(http.StatusNotFound, apierr.Error("NOT_FOUND", "Not found.", "00000000-0000-0000-0000-000000000000"))
	}
	fields := map[string]any{"updatedAt": time.Now()}
	if req.Name != nil {
		fields["name"] = *req.Name
	}
	if req.Description != nil {
		fields["description"] = *req.Description
	}
	if req.URL != nil {
		fields["url"] = *req.URL
	}
	if req.RoleIDs != nil {
		fields["roleIdsThatCanBeUsedThisDecoration"] = *req.RoleIDs
	}
	if err := h.avatarDecoRepo.UpdateFields(req.ID, fields); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	return c.NoContent(http.StatusNoContent)
}
