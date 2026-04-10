package admin

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// --- accounts ---
func (h *Handler) AccountsDelete(c echo.Context) error { return c.NoContent(http.StatusNoContent) }
func (h *Handler) AccountsFindByEmail(c echo.Context) error {
	return c.JSON(http.StatusNotFound, errResp("USER_NOT_FOUND", "User not found.", "a504947-b888-4a99-9f62-8c4a0f3a3dab"))
}

// --- ad ---
func (h *Handler) AdCreate(c echo.Context) error { return c.NoContent(http.StatusNoContent) }
func (h *Handler) AdDelete(c echo.Context) error { return c.NoContent(http.StatusNoContent) }
func (h *Handler) AdList(c echo.Context) error   { return c.JSON(http.StatusOK, []any{}) }
func (h *Handler) AdUpdate(c echo.Context) error { return c.NoContent(http.StatusNoContent) }

// --- avatar-decorations ---
func (h *Handler) AvatarDecorationsCreate(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}
func (h *Handler) AvatarDecorationsDelete(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}
func (h *Handler) AvatarDecorationsList(c echo.Context) error { return c.JSON(http.StatusOK, []any{}) }
func (h *Handler) AvatarDecorationsUpdate(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// --- captcha ---
func (h *Handler) CaptchaCurrent(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{"provider": nil})
}
func (h *Handler) CaptchaSave(c echo.Context) error { return c.NoContent(http.StatusNoContent) }

// --- abuse-report/notification-recipient ---
func (h *Handler) AbuseReportNotificationRecipientCreate(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}
func (h *Handler) AbuseReportNotificationRecipientDelete(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}
func (h *Handler) AbuseReportNotificationRecipientList(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}
func (h *Handler) AbuseReportNotificationRecipientShow(c echo.Context) error {
	return c.JSON(http.StatusNotFound, errResp("NOT_FOUND", "Not found.", "00000000-0000-0000-0000-000000000000"))
}
func (h *Handler) AbuseReportNotificationRecipientUpdate(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// --- single endpoints ---
func (h *Handler) DeleteAccount(c echo.Context) error { return c.NoContent(http.StatusNoContent) }
func (h *Handler) DeleteAllFilesOfUser(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}
func (h *Handler) ForwardAbuseUserReport(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}
func (h *Handler) GetIndexStats(c echo.Context) error { return c.JSON(http.StatusOK, []any{}) }
func (h *Handler) GetTableStats(c echo.Context) error { return c.JSON(http.StatusOK, map[string]any{}) }
func (h *Handler) GetUserIPs(c echo.Context) error    { return c.JSON(http.StatusOK, []any{}) }
func (h *Handler) ResetPassword(c echo.Context) error { return c.NoContent(http.StatusNoContent) }
func (h *Handler) SendEmail(c echo.Context) error     { return c.NoContent(http.StatusNoContent) }
func (h *Handler) ServerInfo(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{
		"machine": "misskey-go", "os": "linux", "node": "n/a",
		"cpu": map[string]any{"model": "unknown", "cores": 0},
		"mem": map[string]any{"total": 0},
		"fs":  map[string]any{"total": 0, "used": 0},
	})
}
func (h *Handler) UnsetUserAvatar(c echo.Context) error { return c.NoContent(http.StatusNoContent) }
func (h *Handler) UnsetUserBanner(c echo.Context) error { return c.NoContent(http.StatusNoContent) }
func (h *Handler) UpdateAbuseUserReport(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}
func (h *Handler) UpdateProxyAccount(c echo.Context) error { return c.NoContent(http.StatusNoContent) }
func (h *Handler) UpdateUserNote(c echo.Context) error     { return c.NoContent(http.StatusNoContent) }

// --- drive ---
func (h *Handler) DriveCleanRemoteFiles(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}
func (h *Handler) DriveCleanup(c echo.Context) error { return c.NoContent(http.StatusNoContent) }
func (h *Handler) DriveFiles(c echo.Context) error   { return c.JSON(http.StatusOK, []any{}) }
func (h *Handler) DriveShowFile(c echo.Context) error {
	return c.JSON(http.StatusNotFound, errResp("NO_SUCH_FILE", "No such file.", "ac4f7b11-1a6e-47e3-bf3d-3dce9a0e07ab"))
}

// --- emoji bulk ops ---
func (h *Handler) EmojiAddAliasesBulk(c echo.Context) error { return c.NoContent(http.StatusNoContent) }
func (h *Handler) EmojiCopy(c echo.Context) error           { return c.NoContent(http.StatusNoContent) }
func (h *Handler) EmojiDeleteBulk(c echo.Context) error     { return c.NoContent(http.StatusNoContent) }
func (h *Handler) EmojiImportZip(c echo.Context) error      { return c.NoContent(http.StatusNoContent) }
func (h *Handler) EmojiListRemote(c echo.Context) error     { return c.JSON(http.StatusOK, []any{}) }
func (h *Handler) EmojiRemoveAliasesBulk(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}
func (h *Handler) EmojiSetAliasesBulk(c echo.Context) error { return c.NoContent(http.StatusNoContent) }
func (h *Handler) EmojiSetCategoryBulk(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}
func (h *Handler) EmojiSetLicenseBulk(c echo.Context) error { return c.NoContent(http.StatusNoContent) }

// --- federation ---
func (h *Handler) FederationDeleteAllFiles(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}
func (h *Handler) FederationRefreshRemoteInstanceMetadata(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}
func (h *Handler) FederationRemoveAllFollowing(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}
func (h *Handler) FederationUpdateInstance(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// --- invite ---
func (h *Handler) InviteCreate(c echo.Context) error { return c.NoContent(http.StatusNoContent) }
func (h *Handler) InviteList(c echo.Context) error   { return c.JSON(http.StatusOK, []any{}) }

// --- promo ---
func (h *Handler) PromoCreate(c echo.Context) error { return c.NoContent(http.StatusNoContent) }

// --- queue ---
func (h *Handler) QueueClear(c echo.Context) error          { return c.NoContent(http.StatusNoContent) }
func (h *Handler) QueueDeliverDelayed(c echo.Context) error { return c.JSON(http.StatusOK, []any{}) }
func (h *Handler) QueueInboxDelayed(c echo.Context) error   { return c.JSON(http.StatusOK, []any{}) }
func (h *Handler) QueueJobs(c echo.Context) error           { return c.JSON(http.StatusOK, []any{}) }
func (h *Handler) QueuePromoteJobs(c echo.Context) error    { return c.NoContent(http.StatusNoContent) }
func (h *Handler) QueueQueueStats(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{})
}
func (h *Handler) QueueQueues(c echo.Context) error    { return c.JSON(http.StatusOK, []any{}) }
func (h *Handler) QueueRemoveJob(c echo.Context) error { return c.NoContent(http.StatusNoContent) }
func (h *Handler) QueueRetryJob(c echo.Context) error  { return c.NoContent(http.StatusNoContent) }
func (h *Handler) QueueShowJob(c echo.Context) error {
	return c.JSON(http.StatusNotFound, errResp("NOT_FOUND", "Not found.", "00000000-0000-0000-0000-000000000000"))
}
func (h *Handler) QueueShowJobLogs(c echo.Context) error { return c.JSON(http.StatusOK, []any{}) }
func (h *Handler) QueueStats(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{
		"deliver": map[string]any{"activeSince": nil, "active": 0, "waiting": 0, "delayed": 0},
		"inbox":   map[string]any{"activeSince": nil, "active": 0, "waiting": 0, "delayed": 0},
	})
}

// --- relays ---
func (h *Handler) RelaysAdd(c echo.Context) error    { return c.NoContent(http.StatusNoContent) }
func (h *Handler) RelaysList(c echo.Context) error   { return c.JSON(http.StatusOK, []any{}) }
func (h *Handler) RelaysRemove(c echo.Context) error { return c.NoContent(http.StatusNoContent) }

// --- system-webhook ---
func (h *Handler) SystemWebhookCreate(c echo.Context) error { return c.NoContent(http.StatusNoContent) }
func (h *Handler) SystemWebhookDelete(c echo.Context) error { return c.NoContent(http.StatusNoContent) }
func (h *Handler) SystemWebhookList(c echo.Context) error   { return c.JSON(http.StatusOK, []any{}) }
func (h *Handler) SystemWebhookShow(c echo.Context) error {
	return c.JSON(http.StatusNotFound, errResp("NOT_FOUND", "Not found.", "00000000-0000-0000-0000-000000000000"))
}
func (h *Handler) SystemWebhookTest(c echo.Context) error   { return c.NoContent(http.StatusNoContent) }
func (h *Handler) SystemWebhookUpdate(c echo.Context) error { return c.NoContent(http.StatusNoContent) }
