package announcements

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// Handler handles announcement-related API endpoints.
type Handler struct {
	repo  repository.AnnouncementRepository
	idGen id.Generator
}

// NewHandler creates a new announcements Handler.
func NewHandler(repo repository.AnnouncementRepository, idGen id.Generator) *Handler {
	return &Handler{repo: repo, idGen: idGen}
}

// List handles POST /api/announcements.
func (h *Handler) List(c echo.Context) error {
	var req struct {
		Limit    int   `json:"limit"`
		Offset   int   `json:"offset"`
		IsActive *bool `json:"isActive"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "Invalid parameters.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	activeOnly := true
	if req.IsActive != nil {
		activeOnly = *req.IsActive
	}
	items, err := h.repo.List(activeOnly, req.Limit, req.Offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}

	// 認証ユーザーがいれば既読情報を付与
	user := middleware.GetUser(c)
	result := make([]map[string]any, 0, len(items))
	for _, a := range items {
		item := packAnnouncement(a, h.idGen)
		if user != nil {
			read, _ := h.repo.IsRead(user.ID, a.ID)
			item["isRead"] = read
		}
		result = append(result, item)
	}
	return c.JSON(http.StatusOK, result)
}

// ReadAnnouncement handles POST /api/i/read-announcement.
func (h *Handler) ReadAnnouncement(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		AnnouncementID string `json:"announcementId"`
	}
	if err := c.Bind(&req); err != nil || req.AnnouncementID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "announcementId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if _, err := h.repo.FindByID(req.AnnouncementID); err != nil {
		return c.JSON(http.StatusNotFound, errResp("NO_SUCH_ANNOUNCEMENT", "No such announcement.", "b57b5e1d-0158-4f8d-bd54-1ab374089a15"))
	}
	already, _ := h.repo.IsRead(user.ID, req.AnnouncementID)
	if already {
		return c.NoContent(http.StatusNoContent)
	}
	read := &model.AnnouncementRead{
		ID:             h.idGen.Generate(time.Now()),
		UserID:         user.ID,
		AnnouncementID: req.AnnouncementID,
	}
	if err := h.repo.MarkRead(read); err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.NoContent(http.StatusNoContent)
}

// --- Admin endpoints ---

// AdminCreate handles POST /api/admin/announcements/create.
func (h *Handler) AdminCreate(c echo.Context) error {
	var req struct {
		Title    string  `json:"title"`
		Text     string  `json:"text"`
		ImageURL *string `json:"imageUrl"`
		Icon     string  `json:"icon"`
		Display  string  `json:"display"`
	}
	if err := c.Bind(&req); err != nil || req.Title == "" || req.Text == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "title and text are required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if req.Icon == "" {
		req.Icon = "info"
	}
	if req.Display == "" {
		req.Display = "normal"
	}
	a := &model.Announcement{
		ID:       h.idGen.Generate(time.Now()),
		Title:    req.Title,
		Text:     req.Text,
		ImageURL: req.ImageURL,
		Icon:     req.Icon,
		Display:  req.Display,
		IsActive: true,
	}
	if err := h.repo.Create(a); err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.JSON(http.StatusOK, a)
}

// AdminUpdate handles POST /api/admin/announcements/update.
func (h *Handler) AdminUpdate(c echo.Context) error {
	var req struct {
		ID       string  `json:"id"`
		Title    *string `json:"title"`
		Text     *string `json:"text"`
		IsActive *bool   `json:"isActive"`
	}
	if err := c.Bind(&req); err != nil || req.ID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "id is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	fields := map[string]any{}
	if req.Title != nil {
		fields["title"] = *req.Title
	}
	if req.Text != nil {
		fields["text"] = *req.Text
	}
	if req.IsActive != nil {
		fields["isActive"] = *req.IsActive
	}
	if err := h.repo.UpdateFields(req.ID, fields); err != nil {
		return c.JSON(http.StatusNotFound, errResp("NO_SUCH_ANNOUNCEMENT", "No such announcement.", "b57b5e1d-0158-4f8d-bd54-1ab374089a15"))
	}
	return c.NoContent(http.StatusNoContent)
}

// AdminDelete handles POST /api/admin/announcements/delete.
func (h *Handler) AdminDelete(c echo.Context) error {
	var req struct {
		ID string `json:"id"`
	}
	if err := c.Bind(&req); err != nil || req.ID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "id is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if err := h.repo.Delete(req.ID); err != nil {
		return c.JSON(http.StatusNotFound, errResp("NO_SUCH_ANNOUNCEMENT", "No such announcement.", "b57b5e1d-0158-4f8d-bd54-1ab374089a15"))
	}
	return c.NoContent(http.StatusNoContent)
}

// AdminList handles POST /api/admin/announcements/list.
func (h *Handler) AdminList(c echo.Context) error {
	var req struct {
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "Invalid parameters.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	items, err := h.repo.List(false, req.Limit, req.Offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.JSON(http.StatusOK, items)
}

func packAnnouncement(a *model.Announcement, idGen id.Generator) map[string]any {
	createdAt := ""
	if t, err := idGen.ParseTime(a.ID); err == nil {
		createdAt = t.UTC().Format("2006-01-02T15:04:05.000Z")
	}
	return map[string]any{
		"id":                     a.ID,
		"createdAt":              createdAt,
		"updatedAt":              a.UpdatedAt,
		"title":                  a.Title,
		"text":                   a.Text,
		"imageUrl":               a.ImageURL,
		"icon":                   a.Icon,
		"display":                a.Display,
		"needConfirmationToRead": a.NeedConfirmationToRead,
		"silence":                a.Silence,
		"forExistingUsers":       a.ForExistingUsers,
		"isActive":               a.IsActive,
	}
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
