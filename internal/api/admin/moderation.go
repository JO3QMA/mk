package admin

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
)

// UnsetUserAvatar handles POST /api/admin/unset-user-avatar.
func (h *Handler) UnsetUserAvatar(c echo.Context) error {
	var req struct {
		UserID string `json:"userId"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return c.NoContent(http.StatusNoContent)
	}
	_ = h.userRepo.UpdateUser(req.UserID, map[string]any{"avatarId": nil, "avatarUrl": nil, "avatarBlurhash": nil})
	return c.NoContent(http.StatusNoContent)
}

// UnsetUserBanner handles POST /api/admin/unset-user-banner.
func (h *Handler) UnsetUserBanner(c echo.Context) error {
	var req struct {
		UserID string `json:"userId"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return c.NoContent(http.StatusNoContent)
	}
	_ = h.userRepo.UpdateUser(req.UserID, map[string]any{"bannerId": nil, "bannerUrl": nil, "bannerBlurhash": nil})
	return c.NoContent(http.StatusNoContent)
}

// UpdateUserNote handles POST /api/admin/update-user-note. user_profile の
// moderationNote 列を更新する。
func (h *Handler) UpdateUserNote(c echo.Context) error {
	var req struct {
		UserID string `json:"userId"`
		Text   string `json:"text"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return c.NoContent(http.StatusNoContent)
	}
	_ = h.userRepo.UpdateProfile(req.UserID, map[string]any{"moderationNote": req.Text})
	return c.NoContent(http.StatusNoContent)
}

// UpdateAbuseUserReport handles POST /api/admin/update-abuse-user-report.
// 該当 abuse report の resolved フラグを true に立てる moderation 操作。
// reportId 欠落 / abuseRepo 未配線時は no-op で 204 を返す。
func (h *Handler) UpdateAbuseUserReport(c echo.Context) error {
	var req struct {
		ReportID string `json:"reportId"`
	}
	_ = c.Bind(&req)
	if req.ReportID != "" && h.abuseRepo != nil {
		_ = h.abuseRepo.UpdateFields(req.ReportID, map[string]any{"resolved": true})
	}
	return c.NoContent(http.StatusNoContent)
}

// DeleteAllFilesOfUser handles POST /api/admin/delete-all-files-of-a-user.
// driveFile レコードを user 単位で一括 DELETE する。S3 等 object storage の
// 物理 file 削除は drive_file の deletion hook (別経路) で扱う想定。
func (h *Handler) DeleteAllFilesOfUser(c echo.Context) error {
	var req struct {
		UserID string `json:"userId"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "userId is required.", "00000000-0000-0000-0000-000000000000"))
	}
	if h.driveFileRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	// 単一の DELETE 文で完結するため同期実行。大量ファイル (数万) の場合も
	// PostgreSQL で 1 秒未満に収まる想定。将来バッチが必要なら queue へ。
	if _, err := h.driveFileRepo.DeleteByUser(req.UserID); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	return c.NoContent(http.StatusNoContent)
}
