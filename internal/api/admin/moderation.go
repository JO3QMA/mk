package admin

import (
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	"github.com/shiroha-a/mk/internal/core/moderationlog"
)

// UnsetUserAvatar handles POST /api/admin/unset-user-avatar.
func (h *Handler) UnsetUserAvatar(c echo.Context) error {
	var req struct {
		UserID string `json:"userId"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return c.NoContent(http.StatusNoContent)
	}
	if err := h.userRepo.UpdateUser(req.UserID, map[string]any{"avatarId": nil, "avatarUrl": nil, "avatarBlurhash": nil}); err == nil {
		h.logUserAction(c, moderationlog.LogUnsetUserAvatar, req.UserID)
	}
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
	if err := h.userRepo.UpdateUser(req.UserID, map[string]any{"bannerId": nil, "bannerUrl": nil, "bannerBlurhash": nil}); err == nil {
		h.logUserAction(c, moderationlog.LogUnsetUserBanner, req.UserID)
	}
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
	// before を log の info に含めるため UpdateProfile の前に取得する。
	// 取得失敗時は before 不明扱い (空文字) で log を書く方が監査価値が高い。
	var before string
	if profile, err := h.userRepo.FindProfileByUserID(req.UserID); err == nil && profile != nil && profile.ModerationNote != nil {
		before = *profile.ModerationNote
	}
	if err := h.userRepo.UpdateProfile(req.UserID, map[string]any{"moderationNote": req.Text}); err != nil {
		return c.NoContent(http.StatusNoContent)
	}
	target, err := h.userRepo.FindByID(req.UserID)
	if err != nil || target == nil {
		// UpdateProfile が成功した直後に user lookup が失敗するのは
		// 並行削除等のレアケース。debug-level に残して早期 return する。
		slog.DebugContext(c.Request().Context(), "moderation log: target user not found after UpdateUserNote",
			"userId", req.UserID)
		return c.NoContent(http.StatusNoContent)
	}
	info := moderationlog.UserInfo(target)
	info["before"] = before
	info["after"] = req.Text
	h.logModeration(c, moderationlog.LogUpdateUserNote, info)
	return c.NoContent(http.StatusNoContent)
}

// UpdateAbuseUserReport handles POST /api/admin/update-abuse-user-report.
// 該当 abuse report の resolved フラグを true に立てる moderation 操作。
// reportId 欠落 / abuseRepo 未配線時は no-op で 204 を返す。
//
// Misskey TS の resolve-abuse-user-report 相当。log type は resolveAbuseReport
// (note 編集分岐は別 endpoint なので本 handler では非対象)。info schema は
// `{reportId, report, resolvedAs}` で TS と互換。mk-go では resolvedAs を
// 受け取らないので nil を入れる。
func (h *Handler) UpdateAbuseUserReport(c echo.Context) error {
	var req struct {
		ReportID string `json:"reportId"`
	}
	_ = c.Bind(&req)
	if req.ReportID == "" || h.abuseRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	// before snapshot for moderation log info
	snapshot, _ := h.abuseRepo.FindByID(req.ReportID)
	if err := h.abuseRepo.UpdateFields(req.ReportID, map[string]any{"resolved": true}); err == nil && snapshot != nil {
		h.logModeration(c, moderationlog.LogResolveAbuseReport, map[string]any{
			"reportId":   req.ReportID,
			"report":     snapshot,
			"resolvedAs": nil,
		})
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
		return c.JSON(http.StatusBadRequest, apierr.InvalidParam("userId is required."))
	}
	if h.driveFileRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	// 単一の DELETE 文で完結するため同期実行。大量ファイル (数万) の場合も
	// PostgreSQL で 1 秒未満に収まる想定。将来バッチが必要なら queue へ。
	if _, err := h.driveFileRepo.DeleteByUser(req.UserID); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.InternalError())
	}
	return c.NoContent(http.StatusNoContent)
}
