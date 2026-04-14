package roles

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	"github.com/shiroha-a/mk/internal/core/role"
	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
)

// RoleNotesQuery provides a way to fetch notes by role.
type RoleNotesQuery interface {
	ListByRole(roleID string, limit int, sinceID, untilID string) ([]*model.Note, error)
}

// Handler handles public role API endpoints.
type Handler struct {
	roleService *role.Service
	notesQuery  RoleNotesQuery
	idGen       id.Generator
}

// NewHandler creates a new roles Handler.
func NewHandler(roleService *role.Service) *Handler {
	return &Handler{roleService: roleService}
}

// SetNotesQuery attaches a RoleNotesQuery for the roles/notes endpoint.
func (h *Handler) SetNotesQuery(q RoleNotesQuery) {
	h.notesQuery = q
}

// SetIDGen attaches an ID generator for note packing.
func (h *Handler) SetIDGen(g id.Generator) {
	h.idGen = g
}

// List handles POST /api/roles/list.
func (h *Handler) List(c echo.Context) error {
	roles, err := h.roleService.List()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
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
	return c.JSON(http.StatusOK, []any{})
}

// Notes handles POST /api/roles/notes.
func (h *Handler) Notes(c echo.Context) error {
	var req struct {
		RoleID  string `json:"roleId"`
		Limit   int    `json:"limit"`
		SinceID string `json:"sinceId"`
		UntilID string `json:"untilId"`
	}
	if err := c.Bind(&req); err != nil || req.RoleID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "roleId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}

	r, err := h.roleService.Show(req.RoleID)
	if err != nil {
		return c.JSON(http.StatusNotFound, errResp("NO_SUCH_ROLE", "No such role.", "07dc7d34-c0d8-458b-9c04-4b18399b1f46"))
	}
	if !r.IsPublic {
		return c.JSON(http.StatusNotFound, errResp("NO_SUCH_ROLE", "No such role.", "07dc7d34-c0d8-458b-9c04-4b18399b1f46"))
	}

	if h.notesQuery == nil {
		return c.JSON(http.StatusOK, []any{})
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	notes, err := h.notesQuery.ListByRole(req.RoleID, limit, req.SinceID, req.UntilID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}

	out := make([]any, 0, len(notes))
	for _, n := range notes {
		out = append(out, entity.PackNote(n, h.idGen))
	}
	return c.JSON(http.StatusOK, out)
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
	return apierr.Error(code, message, id)
}
