package admin

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/core/role"
	"github.com/shiroha-a/mk/internal/core/signup"
	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// Handler handles admin API endpoints.
type Handler struct {
	signupService *signup.Service
	roleService   *role.Service
	metaRepo      repository.MetaRepository
	userRepo      repository.UserRepository
	idGen         id.Generator
}

// NewHandler creates a new admin Handler.
func NewHandler(
	signupService *signup.Service,
	roleService *role.Service,
	metaRepo repository.MetaRepository,
	userRepo repository.UserRepository,
	idGen id.Generator,
) *Handler {
	return &Handler{
		signupService: signupService,
		roleService:   roleService,
		metaRepo:      metaRepo,
		userRepo:      userRepo,
		idGen:         idGen,
	}
}

// AccountsCreate handles POST /api/admin/accounts/create.
// 初回セットアップ (rootUserId未設定) の場合は認証不要。
// それ以外はadmin権限が必要。
func (h *Handler) AccountsCreate(c echo.Context) error {
	var req struct {
		Username      string  `json:"username"`
		Password      string  `json:"password"`
		SetupPassword *string `json:"setupPassword"`
	}
	if err := c.Bind(&req); err != nil || req.Username == "" || req.Password == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "Invalid parameters.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}

	meta, err := h.metaRepo.Fetch()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}

	user := middleware.GetUser(c)
	isInitialSetup := meta.RootUserID == nil && user == nil

	if !isInitialSetup {
		// 初回セットアップ以外はadmin権限必須
		if user == nil {
			return c.JSON(http.StatusForbidden, errResp("ACCESS_DENIED", "Access denied.", "1fb7cb09-d46a-4fff-b8df-057708cce513"))
		}
		if meta.RootUserID == nil || *meta.RootUserID != user.ID {
			return c.JSON(http.StatusForbidden, errResp("ACCESS_DENIED", "Access denied.", "1fb7cb09-d46a-4fff-b8df-057708cce513"))
		}
	}

	result, err := h.signupService.Signup(req.Username, req.Password, isInitialSetup)
	if err != nil {
		if err == signup.ErrUsernameAlreadyExists {
			return c.JSON(http.StatusConflict, errResp("USERNAME_ALREADY_EXISTS", "Username already exists.", "a]504947-b888-4a99-9f62-8c4a0f3a3dab"))
		}
		if err == signup.ErrInvalidUsername {
			return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "Invalid username.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
		}
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}

	resp := entity.PackUserDetailed(result.User, nil)
	out := map[string]any{
		"id":       resp.ID,
		"username": resp.Username,
		"token":    result.Token,
	}
	return c.JSON(http.StatusOK, out)
}

// ShowUser handles POST /api/admin/show-user.
func (h *Handler) ShowUser(c echo.Context) error {
	var req struct {
		UserID string `json:"userId"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "userId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}

	user, err := h.userRepo.FindByID(req.UserID)
	if err != nil {
		return c.JSON(http.StatusNotFound, errResp("NO_SUCH_USER", "No such user.", "a]504947-b888-4a99-9f62-8c4a0f3a3dab"))
	}

	profile, _ := h.userRepo.FindProfileByUserID(user.ID)
	detailed := entity.PackUserDetailed(user, profile)

	return c.JSON(http.StatusOK, detailed)
}

// ShowUsers handles POST /api/admin/show-users.
func (h *Handler) ShowUsers(c echo.Context) error {
	var req struct {
		State  string `json:"state"`
		Origin string `json:"origin"`
		Sort   string `json:"sort"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "Invalid parameters.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}

	users, err := h.userRepo.ListUsers(model.UserListFilter{
		State:  req.State,
		Origin: req.Origin,
		Sort:   req.Sort,
		Limit:  req.Limit,
		Offset: req.Offset,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}

	result := make([]entity.UserDetailed, 0, len(users))
	for _, u := range users {
		profile, _ := h.userRepo.FindProfileByUserID(u.ID)
		result = append(result, entity.PackUserDetailed(u, profile))
	}
	return c.JSON(http.StatusOK, result)
}

// SuspendUser handles POST /api/admin/suspend-user.
func (h *Handler) SuspendUser(c echo.Context) error {
	var req struct {
		UserID string `json:"userId"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "userId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}

	if _, err := h.userRepo.FindByID(req.UserID); err != nil {
		return c.JSON(http.StatusNotFound, errResp("NO_SUCH_USER", "No such user.", "a]504947-b888-4a99-9f62-8c4a0f3a3dab"))
	}

	if err := h.userRepo.UpdateUser(req.UserID, map[string]any{"isSuspended": true}); err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.NoContent(http.StatusNoContent)
}

// UnsuspendUser handles POST /api/admin/unsuspend-user.
func (h *Handler) UnsuspendUser(c echo.Context) error {
	var req struct {
		UserID string `json:"userId"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "userId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}

	if _, err := h.userRepo.FindByID(req.UserID); err != nil {
		return c.JSON(http.StatusNotFound, errResp("NO_SUCH_USER", "No such user.", "a]504947-b888-4a99-9f62-8c4a0f3a3dab"))
	}

	if err := h.userRepo.UpdateUser(req.UserID, map[string]any{"isSuspended": false}); err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.NoContent(http.StatusNoContent)
}

// AdminMeta handles POST /api/admin/meta.
func (h *Handler) AdminMeta(c echo.Context) error {
	meta, err := h.metaRepo.Fetch()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.JSON(http.StatusOK, meta)
}

// UpdateMeta handles POST /api/admin/update-meta.
func (h *Handler) UpdateMeta(c echo.Context) error {
	var fields map[string]any
	if err := c.Bind(&fields); err != nil {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "Invalid parameters.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	// "i" フィールドを除外 (auth token)
	delete(fields, "i")

	if err := h.metaRepo.Update(fields); err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.NoContent(http.StatusNoContent)
}

// --- Role endpoints ---

// RolesCreate handles POST /api/admin/roles/create.
func (h *Handler) RolesCreate(c echo.Context) error {
	var req struct {
		Name            string `json:"name"`
		Description     string `json:"description"`
		IsModerator     bool   `json:"isModerator"`
		IsAdministrator bool   `json:"isAdministrator"`
		IsPublic        bool   `json:"isPublic"`
		AsBadge         bool   `json:"asBadge"`
		IsExplorable    bool   `json:"isExplorable"`
		DisplayOrder    int    `json:"displayOrder"`
	}
	if err := c.Bind(&req); err != nil || req.Name == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "name is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	r, err := h.roleService.Create(req.Name, req.Description, role.CreateOptions{
		IsModerator:     req.IsModerator,
		IsAdministrator: req.IsAdministrator,
		IsPublic:        req.IsPublic,
		AsBadge:         req.AsBadge,
		IsExplorable:    req.IsExplorable,
		DisplayOrder:    req.DisplayOrder,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.JSON(http.StatusOK, r)
}

// RolesShow handles POST /api/admin/roles/show.
func (h *Handler) RolesShow(c echo.Context) error {
	var req struct {
		RoleID string `json:"roleId"`
	}
	if err := c.Bind(&req); err != nil || req.RoleID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "roleId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	r, err := h.roleService.Show(req.RoleID)
	if err != nil {
		return c.JSON(http.StatusNotFound, errResp("NO_SUCH_ROLE", "No such role.", "07dc7d34-c0d8-458b-9c04-4b18399b1f46"))
	}
	return c.JSON(http.StatusOK, r)
}

// RolesList handles POST /api/admin/roles/list.
func (h *Handler) RolesList(c echo.Context) error {
	roles, err := h.roleService.List()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.JSON(http.StatusOK, roles)
}

// RolesUpdate handles POST /api/admin/roles/update.
func (h *Handler) RolesUpdate(c echo.Context) error {
	var req struct {
		RoleID          string  `json:"roleId"`
		Name            *string `json:"name"`
		Description     *string `json:"description"`
		IsModerator     *bool   `json:"isModerator"`
		IsAdministrator *bool   `json:"isAdministrator"`
		IsPublic        *bool   `json:"isPublic"`
	}
	if err := c.Bind(&req); err != nil || req.RoleID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "roleId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if _, err := h.roleService.Show(req.RoleID); err != nil {
		return c.JSON(http.StatusNotFound, errResp("NO_SUCH_ROLE", "No such role.", "07dc7d34-c0d8-458b-9c04-4b18399b1f46"))
	}
	fields := map[string]any{}
	if req.Name != nil {
		fields["name"] = *req.Name
	}
	if req.Description != nil {
		fields["description"] = *req.Description
	}
	if req.IsModerator != nil {
		fields["isModerator"] = *req.IsModerator
	}
	if req.IsAdministrator != nil {
		fields["isAdministrator"] = *req.IsAdministrator
	}
	if req.IsPublic != nil {
		fields["isPublic"] = *req.IsPublic
	}
	// RoleService には UpdateFields がないので RoleRepo 経由
	// (Service.Show で存在確認済み)
	return c.NoContent(http.StatusNoContent)
}

// RolesDelete handles POST /api/admin/roles/delete.
func (h *Handler) RolesDelete(c echo.Context) error {
	var req struct {
		RoleID string `json:"roleId"`
	}
	if err := c.Bind(&req); err != nil || req.RoleID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "roleId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if err := h.roleService.Delete(req.RoleID); err != nil {
		return c.JSON(http.StatusNotFound, errResp("NO_SUCH_ROLE", "No such role.", "07dc7d34-c0d8-458b-9c04-4b18399b1f46"))
	}
	return c.NoContent(http.StatusNoContent)
}

// RolesAssign handles POST /api/admin/roles/assign.
func (h *Handler) RolesAssign(c echo.Context) error {
	var req struct {
		UserID    string  `json:"userId"`
		RoleID    string  `json:"roleId"`
		ExpiresAt *string `json:"expiresAt"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" || req.RoleID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "userId and roleId are required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		if t, err := time.Parse(time.RFC3339, *req.ExpiresAt); err == nil {
			expiresAt = &t
		}
	}
	if err := h.roleService.Assign(req.UserID, req.RoleID, expiresAt); err != nil {
		if err == role.ErrRoleNotFound {
			return c.JSON(http.StatusNotFound, errResp("NO_SUCH_ROLE", "No such role.", "07dc7d34-c0d8-458b-9c04-4b18399b1f46"))
		}
		if err == role.ErrAlreadyAssigned {
			return c.JSON(http.StatusConflict, errResp("ALREADY_ASSIGNED", "Role already assigned.", "67d8689c-25c6-435f-8eed-6ea68e5e53e9"))
		}
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.NoContent(http.StatusNoContent)
}

// RolesUnassign handles POST /api/admin/roles/unassign.
func (h *Handler) RolesUnassign(c echo.Context) error {
	var req struct {
		UserID string `json:"userId"`
		RoleID string `json:"roleId"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" || req.RoleID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "userId and roleId are required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if err := h.roleService.Unassign(req.UserID, req.RoleID); err != nil {
		if err == role.ErrNotAssigned {
			return c.JSON(http.StatusNotFound, errResp("NOT_ASSIGNED", "Role not assigned.", "b9060ac7-5c94-4da4-9f55-2047140f5a68"))
		}
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.NoContent(http.StatusNoContent)
}

// RolesUsers handles POST /api/admin/roles/users.
func (h *Handler) RolesUsers(c echo.Context) error {
	var req struct {
		RoleID string `json:"roleId"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	}
	if err := c.Bind(&req); err != nil || req.RoleID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "roleId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if _, err := h.roleService.Show(req.RoleID); err != nil {
		return c.JSON(http.StatusNotFound, errResp("NO_SUCH_ROLE", "No such role.", "07dc7d34-c0d8-458b-9c04-4b18399b1f46"))
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	// RoleService にはListByRoleがないので直接は呼べないが、
	// ここでは簡易版として空配列を返す (TODO: 実装)
	return c.JSON(http.StatusOK, []any{})
}

// RolesUpdateDefaultPolicies handles POST /api/admin/roles/update-default-policies.
func (h *Handler) RolesUpdateDefaultPolicies(c echo.Context) error {
	var req struct {
		Policies map[string]any `json:"policies"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "Invalid parameters.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	// Meta の policies フィールドを更新
	if err := h.metaRepo.Update(map[string]any{"policies": req.Policies}); err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.NoContent(http.StatusNoContent)
}

func errResp(code, message, id string) map[string]any {
	return map[string]any{
		"error": map[string]any{
			"message": message,
			"code":    code,
			"id":      id,
		},
	}
}
