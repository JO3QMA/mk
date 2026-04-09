package roles

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/core/role"
	"github.com/shiroha-a/mk/internal/model"
)

// Handler handles public role API endpoints.
type Handler struct {
	roleService *role.Service
}

// NewHandler creates a new roles Handler.
func NewHandler(roleService *role.Service) *Handler {
	return &Handler{roleService: roleService}
}

// List handles POST /api/roles/list.
func (h *Handler) List(c echo.Context) error {
	roles, err := h.roleService.List()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	// 公開ロールのみフィルタ
	var result []any
	for _, r := range roles {
		if r.IsPublic {
			result = append(result, packRole(r))
		}
	}
	if result == nil {
		result = []any{}
	}
	return c.JSON(http.StatusOK, result)
}

// Show handles POST /api/roles/show.
func (h *Handler) Show(c echo.Context) error {
	var req struct {
		RoleID string `json:"roleId"`
	}
	if err := c.Bind(&req); err != nil || req.RoleID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "roleId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	r, err := h.roleService.Show(req.RoleID)
	if err != nil || !r.IsPublic {
		return c.JSON(http.StatusNotFound, errResp("NO_SUCH_ROLE", "No such role.", "07dc7d34-c0d8-458b-9c04-4b18399b1f46"))
	}
	return c.JSON(http.StatusOK, packRole(r))
}

// Users handles POST /api/roles/users.
func (h *Handler) Users(c echo.Context) error {
	var req struct {
		RoleID string `json:"roleId"`
	}
	if err := c.Bind(&req); err != nil || req.RoleID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "roleId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if _, err := h.roleService.Show(req.RoleID); err != nil {
		return c.JSON(http.StatusNotFound, errResp("NO_SUCH_ROLE", "No such role.", "07dc7d34-c0d8-458b-9c04-4b18399b1f46"))
	}
	// ユーザー一覧は簡易版 (TODO: RoleService.ListByRole)
	return c.JSON(http.StatusOK, []any{})
}

func packRole(r *model.Role) map[string]any {
	return map[string]any{
		"id":              r.ID,
		"name":            r.Name,
		"color":           r.Color,
		"iconUrl":         r.IconURL,
		"description":     r.Description,
		"isModerator":     r.IsModerator,
		"isAdministrator": r.IsAdministrator,
		"isPublic":        r.IsPublic,
		"isExplorable":    r.IsExplorable,
		"asBadge":         r.AsBadge,
		"displayOrder":    r.DisplayOrder,
	}
}

func errResp(code, message, id string) map[string]any {
	return map[string]any{
		"error": map[string]any{"message": message, "code": code, "id": id},
	}
}
