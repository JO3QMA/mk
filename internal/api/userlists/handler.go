package userlists

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// Handler handles users/lists API endpoints.
type Handler struct {
	repo  repository.UserListRepository
	idGen id.Generator
}

// NewHandler creates a new userlists Handler.
func NewHandler(repo repository.UserListRepository, idGen id.Generator) *Handler {
	return &Handler{repo: repo, idGen: idGen}
}

// List handles POST /api/users/lists/list.
func (h *Handler) List(c echo.Context) error {
	user := middleware.GetUser(c)
	lists, err := h.repo.ListByUser(user.ID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.JSON(http.StatusOK, lists)
}

// Create handles POST /api/users/lists/create.
func (h *Handler) Create(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		Name string `json:"name"`
	}
	if err := c.Bind(&req); err != nil || req.Name == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "name is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	list := &model.UserList{
		ID:     h.idGen.Generate(time.Now()),
		UserID: user.ID,
		Name:   req.Name,
	}
	if err := h.repo.Create(list); err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.JSON(http.StatusOK, list)
}

// Show handles POST /api/users/lists/show.
func (h *Handler) Show(c echo.Context) error {
	var req struct {
		ListID string `json:"listId"`
	}
	if err := c.Bind(&req); err != nil || req.ListID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "listId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	list, err := h.repo.FindByID(req.ListID)
	if err != nil {
		return c.JSON(http.StatusNotFound, errResp("NO_SUCH_LIST", "No such list.", "7bc05c21-1d7a-41ae-88f1-66820f4dc686"))
	}
	return c.JSON(http.StatusOK, list)
}

// Push handles POST /api/users/lists/push (add member).
func (h *Handler) Push(c echo.Context) error {
	var req struct {
		ListID string `json:"listId"`
		UserID string `json:"userId"`
	}
	if err := c.Bind(&req); err != nil || req.ListID == "" || req.UserID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "listId and userId are required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if _, err := h.repo.FindByID(req.ListID); err != nil {
		return c.JSON(http.StatusNotFound, errResp("NO_SUCH_LIST", "No such list.", "7bc05c21-1d7a-41ae-88f1-66820f4dc686"))
	}
	m := &model.UserListMembership{
		ID:         h.idGen.Generate(time.Now()),
		UserListID: req.ListID,
		UserID:     req.UserID,
	}
	if err := h.repo.AddMember(m); err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.NoContent(http.StatusNoContent)
}

// Pull handles POST /api/users/lists/pull (remove member).
func (h *Handler) Pull(c echo.Context) error {
	var req struct {
		ListID string `json:"listId"`
		UserID string `json:"userId"`
	}
	if err := c.Bind(&req); err != nil || req.ListID == "" || req.UserID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "listId and userId are required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if err := h.repo.RemoveMember(req.ListID, req.UserID); err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.NoContent(http.StatusNoContent)
}

// Delete handles POST /api/users/lists/delete.
func (h *Handler) Delete(c echo.Context) error {
	var req struct {
		ListID string `json:"listId"`
	}
	if err := c.Bind(&req); err != nil || req.ListID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "listId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if err := h.repo.Delete(req.ListID); err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.NoContent(http.StatusNoContent)
}

func errResp(code, message, id string) map[string]any {
	return apierr.Error(code, message, id)
}
