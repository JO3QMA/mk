package i

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	"github.com/shiroha-a/mk/internal/core/transfer"
	"github.com/shiroha-a/mk/internal/queue"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// TransferEnqueuer is the subset of queue.Enqueuer needed to schedule
// export/import jobs. 小さいインターフェースにすることで i/Handler のテスト
// mockが簡単になる。
type TransferEnqueuer interface {
	EnqueueExport(payload queue.ExportPayload) error
	EnqueueImport(payload queue.ImportPayload) error
}

// SetTransferEnqueuer attaches a TransferEnqueuer for i/export-* and
// i/import-* endpoints.
func (h *Handler) SetTransferEnqueuer(e TransferEnqueuer) {
	h.transferEnqueuer = e
}

// exportHandler enqueues an export job for the authenticated user and
// returns 204 No Content on success, matching Misskey's behavior.
func (h *Handler) exportHandler(exportType string) echo.HandlerFunc {
	return func(c echo.Context) error {
		u := middleware.GetUser(c)
		if h.transferEnqueuer == nil {
			return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Transfer queue not configured.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
		}
		if err := h.transferEnqueuer.EnqueueExport(queue.ExportPayload{
			UserID: u.ID,
			Type:   exportType,
		}); err != nil {
			return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Failed to enqueue export.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
		}
		return c.NoContent(http.StatusNoContent)
	}
}

// importHandler enqueues an import job using a DriveFile uploaded by the
// user. フロントは fileId を渡してくる。本家互換。
func (h *Handler) importHandler(importType string) echo.HandlerFunc {
	return func(c echo.Context) error {
		u := middleware.GetUser(c)
		var req struct {
			FileID string `json:"fileId"`
		}
		if err := c.Bind(&req); err != nil || req.FileID == "" {
			return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "fileId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
		}
		if h.transferEnqueuer == nil {
			return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Transfer queue not configured.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
		}
		if err := h.transferEnqueuer.EnqueueImport(queue.ImportPayload{
			UserID: u.ID,
			Type:   importType,
			FileID: req.FileID,
		}); err != nil {
			return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Failed to enqueue import.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
		}
		return c.NoContent(http.StatusNoContent)
	}
}

// Export handler factory methods — one per endpoint for router binding.
func (h *Handler) ExportNotes(c echo.Context) error {
	return h.exportHandler(transfer.ExportNotes)(c)
}
func (h *Handler) ExportFollowing(c echo.Context) error {
	return h.exportHandler(transfer.ExportFollowing)(c)
}
func (h *Handler) ExportBlocking(c echo.Context) error {
	return h.exportHandler(transfer.ExportBlocking)(c)
}
func (h *Handler) ExportMute(c echo.Context) error {
	return h.exportHandler(transfer.ExportMuting)(c)
}
func (h *Handler) ExportFavorites(c echo.Context) error {
	return h.exportHandler(transfer.ExportFavorites)(c)
}
func (h *Handler) ExportUserLists(c echo.Context) error {
	return h.exportHandler(transfer.ExportUserLists)(c)
}
func (h *Handler) ExportAntennas(c echo.Context) error {
	return h.exportHandler(transfer.ExportAntennas)(c)
}
func (h *Handler) ExportClips(c echo.Context) error {
	return h.exportHandler(transfer.ExportClips)(c)
}

// Import handler factory methods.
func (h *Handler) ImportFollowing(c echo.Context) error {
	return h.importHandler(transfer.ImportFollowing)(c)
}
func (h *Handler) ImportBlocking(c echo.Context) error {
	return h.importHandler(transfer.ImportBlocking)(c)
}
func (h *Handler) ImportMuting(c echo.Context) error {
	return h.importHandler(transfer.ImportMuting)(c)
}
func (h *Handler) ImportUserLists(c echo.Context) error {
	return h.importHandler(transfer.ImportUserLists)(c)
}
func (h *Handler) ImportAntennas(c echo.Context) error {
	return h.importHandler(transfer.ImportAntennas)(c)
}
