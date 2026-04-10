package announcements

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Show handles POST /api/announcements/show.
func (h *Handler) Show(c echo.Context) error {
	var req struct {
		AnnouncementID string `json:"announcementId"`
	}
	if err := c.Bind(&req); err != nil || req.AnnouncementID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "announcementId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	a, err := h.repo.FindByID(req.AnnouncementID)
	if err != nil {
		return c.JSON(http.StatusNotFound, errResp("NO_SUCH_ANNOUNCEMENT", "No such announcement.", "e0ef2076-ee94-4ebc-a2c8-63bec9e7ab72"))
	}
	return c.JSON(http.StatusOK, packAnnouncement(a, h.idGen))
}
