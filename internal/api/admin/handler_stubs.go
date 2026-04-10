package admin

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/server/middleware"
	"golang.org/x/crypto/bcrypt"
)

// --- accounts ---

// AccountsDelete handles POST /api/admin/accounts/delete.
func (h *Handler) AccountsDelete(c echo.Context) error {
	var req struct {
		UserID string `json:"userId"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return c.NoContent(http.StatusNoContent)
	}
	_ = h.userRepo.UpdateUser(req.UserID, map[string]any{"isSuspended": true, "isDeleted": true})
	return c.NoContent(http.StatusNoContent)
}

// AccountsFindByEmail handles POST /api/admin/accounts/find-by-email.
func (h *Handler) AccountsFindByEmail(c echo.Context) error {
	var req struct {
		Email string `json:"email"`
	}
	if err := c.Bind(&req); err != nil || req.Email == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "email is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	// メール検索は未実装 (user_profileテーブルのemail列検索が必要)
	return c.JSON(http.StatusNotFound, errResp("USER_NOT_FOUND", "User not found.", "a504947-b888-4a99-9f62-8c4a0f3a3dab"))
}

// --- single endpoints ---

// DeleteAccount handles POST /api/admin/delete-account.
func (h *Handler) DeleteAccount(c echo.Context) error {
	var req struct {
		UserID string `json:"userId"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return c.NoContent(http.StatusNoContent)
	}
	_ = h.userRepo.UpdateUser(req.UserID, map[string]any{"isSuspended": true, "isDeleted": true})
	return c.NoContent(http.StatusNoContent)
}

// DeleteAllFilesOfUser handles POST /api/admin/delete-all-files-of-a-user.
func (h *Handler) DeleteAllFilesOfUser(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// ForwardAbuseUserReport handles POST /api/admin/forward-abuse-user-report.
func (h *Handler) ForwardAbuseUserReport(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// GetIndexStats handles POST /api/admin/get-index-stats.
func (h *Handler) GetIndexStats(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

// GetTableStats handles POST /api/admin/get-table-stats.
func (h *Handler) GetTableStats(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{})
}

// GetUserIPs handles POST /api/admin/get-user-ips.
func (h *Handler) GetUserIPs(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

// ResetPassword handles POST /api/admin/reset-password.
func (h *Handler) ResetPassword(c echo.Context) error {
	var req struct {
		UserID string `json:"userId"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "userId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	// ランダムパスワード生成
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	newPass := hex.EncodeToString(b)
	hash, _ := bcrypt.GenerateFromPassword([]byte(newPass), bcrypt.DefaultCost)
	_ = h.userRepo.UpdateProfile(req.UserID, map[string]any{"password": string(hash)})
	return c.JSON(http.StatusOK, map[string]any{"password": newPass})
}

// SendEmail handles POST /api/admin/send-email.
func (h *Handler) SendEmail(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// ServerInfo handles POST /api/admin/server-info.
func (h *Handler) ServerInfo(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{
		"machine": "misskey-go", "os": "linux", "node": "n/a",
		"cpu": map[string]any{"model": "unknown", "cores": 0},
		"mem": map[string]any{"total": 0},
		"fs":  map[string]any{"total": 0, "used": 0},
	})
}

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

// UpdateAbuseUserReport handles POST /api/admin/update-abuse-user-report.
func (h *Handler) UpdateAbuseUserReport(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// UpdateProxyAccount handles POST /api/admin/update-proxy-account.
func (h *Handler) UpdateProxyAccount(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// UpdateUserNote handles POST /api/admin/update-user-note.
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

// --- drive ---

// DriveCleanRemoteFiles handles POST /api/admin/drive/clean-remote-files.
func (h *Handler) DriveCleanRemoteFiles(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// DriveCleanup handles POST /api/admin/drive/cleanup.
func (h *Handler) DriveCleanup(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// DriveFiles handles POST /api/admin/drive/files.
func (h *Handler) DriveFiles(c echo.Context) error {
	if h.driveFileRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	var req struct {
		Limit   int    `json:"limit"`
		SinceID string `json:"sinceId"`
		UntilID string `json:"untilId"`
	}
	_ = c.Bind(&req)
	if req.Limit <= 0 {
		req.Limit = 10
	}
	// 全ファイル一覧（管理者用）
	files, err := h.driveFileRepo.ListByUser("", nil, req.UntilID, req.SinceID, req.Limit)
	if err != nil {
		return c.JSON(http.StatusOK, []any{})
	}
	out := make([]entity.DriveFileEntity, 0, len(files))
	for _, f := range files {
		out = append(out, entity.PackDriveFile(f, h.idGen))
	}
	return c.JSON(http.StatusOK, out)
}

// DriveShowFile handles POST /api/admin/drive/show-file.
func (h *Handler) DriveShowFile(c echo.Context) error {
	var req struct {
		FileID string `json:"fileId"`
	}
	if err := c.Bind(&req); err != nil || req.FileID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "fileId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if h.driveFileRepo == nil {
		return c.JSON(http.StatusNotFound, errResp("NO_SUCH_FILE", "No such file.", "ac4f7b11-1a6e-47e3-bf3d-3dce9a0e07ab"))
	}
	file, err := h.driveFileRepo.FindByID(req.FileID)
	if err != nil {
		return c.JSON(http.StatusNotFound, errResp("NO_SUCH_FILE", "No such file.", "ac4f7b11-1a6e-47e3-bf3d-3dce9a0e07ab"))
	}
	return c.JSON(http.StatusOK, entity.PackDriveFile(file, h.idGen))
}

// --- emoji bulk ops ---

// EmojiAddAliasesBulk handles POST /api/admin/emoji/add-aliases-bulk.
func (h *Handler) EmojiAddAliasesBulk(c echo.Context) error {
	var req struct {
		IDs     []string `json:"ids"`
		Aliases []string `json:"aliases"`
	}
	if err := c.Bind(&req); err != nil || len(req.IDs) == 0 {
		return c.NoContent(http.StatusNoContent)
	}
	if h.emojiRepo != nil {
		for _, eid := range req.IDs {
			if e, err := h.emojiRepo.FindByID(eid); err == nil {
				merged := append(e.Aliases, req.Aliases...)
				_ = h.emojiRepo.UpdateFields(eid, map[string]any{"aliases": merged})
			}
		}
	}
	return c.NoContent(http.StatusNoContent)
}

// EmojiCopy handles POST /api/admin/emoji/copy.
func (h *Handler) EmojiCopy(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// EmojiDeleteBulk handles POST /api/admin/emoji/delete-bulk.
func (h *Handler) EmojiDeleteBulk(c echo.Context) error {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := c.Bind(&req); err != nil || len(req.IDs) == 0 {
		return c.NoContent(http.StatusNoContent)
	}
	if h.emojiRepo != nil {
		for _, eid := range req.IDs {
			_ = h.emojiRepo.Delete(eid)
		}
	}
	return c.NoContent(http.StatusNoContent)
}

// EmojiImportZip handles POST /api/admin/emoji/import-zip.
func (h *Handler) EmojiImportZip(c echo.Context) error {
	// ZIP展開+インポートは将来対応
	return c.NoContent(http.StatusNoContent)
}

// EmojiListRemote handles POST /api/admin/emoji/list-remote.
func (h *Handler) EmojiListRemote(c echo.Context) error {
	if h.emojiRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	emojis, err := h.emojiRepo.ListWithFilter("", "", false, 20, 0)
	if err != nil {
		return c.JSON(http.StatusOK, []any{})
	}
	return c.JSON(http.StatusOK, emojis)
}

// EmojiRemoveAliasesBulk handles POST /api/admin/emoji/remove-aliases-bulk.
func (h *Handler) EmojiRemoveAliasesBulk(c echo.Context) error {
	var req struct {
		IDs     []string `json:"ids"`
		Aliases []string `json:"aliases"`
	}
	if err := c.Bind(&req); err != nil || len(req.IDs) == 0 {
		return c.NoContent(http.StatusNoContent)
	}
	if h.emojiRepo != nil {
		removeSet := make(map[string]bool, len(req.Aliases))
		for _, a := range req.Aliases {
			removeSet[a] = true
		}
		for _, eid := range req.IDs {
			if e, err := h.emojiRepo.FindByID(eid); err == nil {
				var filtered []string
				for _, a := range e.Aliases {
					if !removeSet[a] {
						filtered = append(filtered, a)
					}
				}
				_ = h.emojiRepo.UpdateFields(eid, map[string]any{"aliases": filtered})
			}
		}
	}
	return c.NoContent(http.StatusNoContent)
}

// EmojiSetAliasesBulk handles POST /api/admin/emoji/set-aliases-bulk.
func (h *Handler) EmojiSetAliasesBulk(c echo.Context) error {
	var req struct {
		IDs     []string `json:"ids"`
		Aliases []string `json:"aliases"`
	}
	if err := c.Bind(&req); err != nil || len(req.IDs) == 0 {
		return c.NoContent(http.StatusNoContent)
	}
	if h.emojiRepo != nil {
		for _, eid := range req.IDs {
			_ = h.emojiRepo.UpdateFields(eid, map[string]any{"aliases": req.Aliases})
		}
	}
	return c.NoContent(http.StatusNoContent)
}

// EmojiSetCategoryBulk handles POST /api/admin/emoji/set-category-bulk.
func (h *Handler) EmojiSetCategoryBulk(c echo.Context) error {
	var req struct {
		IDs      []string `json:"ids"`
		Category string   `json:"category"`
	}
	if err := c.Bind(&req); err != nil || len(req.IDs) == 0 {
		return c.NoContent(http.StatusNoContent)
	}
	if h.emojiRepo != nil {
		for _, eid := range req.IDs {
			_ = h.emojiRepo.UpdateFields(eid, map[string]any{"category": req.Category})
		}
	}
	return c.NoContent(http.StatusNoContent)
}

// EmojiSetLicenseBulk handles POST /api/admin/emoji/set-license-bulk.
func (h *Handler) EmojiSetLicenseBulk(c echo.Context) error {
	var req struct {
		IDs     []string `json:"ids"`
		License string   `json:"license"`
	}
	if err := c.Bind(&req); err != nil || len(req.IDs) == 0 {
		return c.NoContent(http.StatusNoContent)
	}
	if h.emojiRepo != nil {
		for _, eid := range req.IDs {
			_ = h.emojiRepo.UpdateFields(eid, map[string]any{"license": req.License})
		}
	}
	return c.NoContent(http.StatusNoContent)
}

// --- captcha ---

// CaptchaCurrent handles POST /api/admin/captcha/current.
func (h *Handler) CaptchaCurrent(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{"provider": nil})
}

// CaptchaSave handles POST /api/admin/captcha/save.
func (h *Handler) CaptchaSave(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// --- ad ---

// AdCreate handles POST /api/admin/ad/create.
func (h *Handler) AdCreate(c echo.Context) error {
	if h.adminDB == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		URL      string `json:"url"`
		ImageURL string `json:"imageUrl"`
		Place    string `json:"place"`
		Memo     string `json:"memo"`
		Priority string `json:"priority"`
		Ratio    int    `json:"ratio"`
	}
	_ = c.Bind(&req)
	if req.Place == "" {
		req.Place = "square"
	}
	if req.Priority == "" {
		req.Priority = "middle"
	}
	if req.Ratio <= 0 {
		req.Ratio = 1
	}
	ad := &model.Ad{
		ID: h.idGen.Generate(time.Now()), URL: req.URL, ImageURL: req.ImageURL,
		Place: req.Place, Memo: req.Memo, Priority: req.Priority, Ratio: req.Ratio,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour), StartsAt: time.Now(),
	}
	h.adminDB.Create(ad)
	return c.JSON(http.StatusOK, ad)
}

// AdDelete handles POST /api/admin/ad/delete.
func (h *Handler) AdDelete(c echo.Context) error {
	if h.adminDB == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		ID string `json:"id"`
	}
	_ = c.Bind(&req)
	h.adminDB.Where(`"id" = ?`, req.ID).Delete(&model.Ad{})
	return c.NoContent(http.StatusNoContent)
}

// AdList handles POST /api/admin/ad/list.
func (h *Handler) AdList(c echo.Context) error {
	if h.adminDB == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	var ads []*model.Ad
	h.adminDB.Order(`"id" DESC`).Limit(20).Find(&ads)
	return c.JSON(http.StatusOK, ads)
}

// AdUpdate handles POST /api/admin/ad/update.
func (h *Handler) AdUpdate(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// --- avatar-decorations ---

// AvatarDecorationsCreate handles POST /api/admin/avatar-decorations/create.
func (h *Handler) AvatarDecorationsCreate(c echo.Context) error {
	if h.adminDB == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		URL         string `json:"url"`
	}
	_ = c.Bind(&req)
	d := &model.AvatarDecoration{
		ID: h.idGen.Generate(time.Now()), Name: req.Name, Description: req.Description, URL: req.URL,
	}
	h.adminDB.Create(d)
	return c.JSON(http.StatusOK, d)
}

// AvatarDecorationsDelete handles POST /api/admin/avatar-decorations/delete.
func (h *Handler) AvatarDecorationsDelete(c echo.Context) error {
	if h.adminDB == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		ID string `json:"id"`
	}
	_ = c.Bind(&req)
	h.adminDB.Where(`"id" = ?`, req.ID).Delete(&model.AvatarDecoration{})
	return c.NoContent(http.StatusNoContent)
}

// AvatarDecorationsList handles POST /api/admin/avatar-decorations/list.
func (h *Handler) AvatarDecorationsList(c echo.Context) error {
	if h.adminDB == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	var decorations []*model.AvatarDecoration
	h.adminDB.Order(`"id" DESC`).Find(&decorations)
	return c.JSON(http.StatusOK, decorations)
}

// AvatarDecorationsUpdate handles POST /api/admin/avatar-decorations/update.
func (h *Handler) AvatarDecorationsUpdate(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// --- abuse-report/notification-recipient ---

// AbuseReportNotificationRecipientCreate handles POST /api/admin/abuse-report/notification-recipient/create.
func (h *Handler) AbuseReportNotificationRecipientCreate(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// AbuseReportNotificationRecipientDelete handles POST /api/admin/abuse-report/notification-recipient/delete.
func (h *Handler) AbuseReportNotificationRecipientDelete(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// AbuseReportNotificationRecipientList handles POST /api/admin/abuse-report/notification-recipient/list.
func (h *Handler) AbuseReportNotificationRecipientList(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

// AbuseReportNotificationRecipientShow handles POST /api/admin/abuse-report/notification-recipient/show.
func (h *Handler) AbuseReportNotificationRecipientShow(c echo.Context) error {
	return c.JSON(http.StatusNotFound, errResp("NOT_FOUND", "Not found.", "00000000-0000-0000-0000-000000000000"))
}

// AbuseReportNotificationRecipientUpdate handles POST /api/admin/abuse-report/notification-recipient/update.
func (h *Handler) AbuseReportNotificationRecipientUpdate(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// --- federation ---

// FederationDeleteAllFiles handles POST /api/admin/federation/delete-all-files.
func (h *Handler) FederationDeleteAllFiles(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// FederationRefreshRemoteInstanceMetadata handles POST /api/admin/federation/refresh-remote-instance-metadata.
func (h *Handler) FederationRefreshRemoteInstanceMetadata(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// FederationRemoveAllFollowing handles POST /api/admin/federation/remove-all-following.
func (h *Handler) FederationRemoveAllFollowing(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// FederationUpdateInstance handles POST /api/admin/federation/update-instance.
func (h *Handler) FederationUpdateInstance(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// --- invite ---

// InviteCreate handles POST /api/admin/invite/create.
func (h *Handler) InviteCreate(c echo.Context) error {
	if h.adminDB == nil {
		return c.NoContent(http.StatusNoContent)
	}
	user := middleware.GetUser(c)
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	code := hex.EncodeToString(b)
	ticket := &model.RegistrationTicket{
		ID: h.idGen.Generate(time.Now()), Code: code, CreatedByID: &user.ID,
	}
	h.adminDB.Create(ticket)
	return c.JSON(http.StatusOK, map[string]any{
		"id": ticket.ID, "code": ticket.Code, "expiresAt": ticket.ExpiresAt,
		"createdAt": time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
	})
}

// InviteList handles POST /api/admin/invite/list.
func (h *Handler) InviteList(c echo.Context) error {
	if h.adminDB == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	var tickets []*model.RegistrationTicket
	h.adminDB.Order(`"id" DESC`).Limit(20).Find(&tickets)
	return c.JSON(http.StatusOK, tickets)
}

// --- promo ---

// PromoCreate handles POST /api/admin/promo/create.
func (h *Handler) PromoCreate(c echo.Context) error { return c.NoContent(http.StatusNoContent) }

// --- queue ---

// QueueClear handles POST /api/admin/queue/clear.
func (h *Handler) QueueClear(c echo.Context) error { return c.NoContent(http.StatusNoContent) }

// QueueDeliverDelayed handles POST /api/admin/queue/deliver-delayed.
func (h *Handler) QueueDeliverDelayed(c echo.Context) error { return c.JSON(http.StatusOK, []any{}) }

// QueueInboxDelayed handles POST /api/admin/queue/inbox-delayed.
func (h *Handler) QueueInboxDelayed(c echo.Context) error { return c.JSON(http.StatusOK, []any{}) }

// QueueJobs handles POST /api/admin/queue/jobs.
func (h *Handler) QueueJobs(c echo.Context) error { return c.JSON(http.StatusOK, []any{}) }

// QueuePromoteJobs handles POST /api/admin/queue/promote-jobs.
func (h *Handler) QueuePromoteJobs(c echo.Context) error { return c.NoContent(http.StatusNoContent) }

// QueueQueueStats handles POST /api/admin/queue/queue-stats.
func (h *Handler) QueueQueueStats(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{})
}

// QueueQueues handles POST /api/admin/queue/queues.
func (h *Handler) QueueQueues(c echo.Context) error { return c.JSON(http.StatusOK, []any{}) }

// QueueRemoveJob handles POST /api/admin/queue/remove-job.
func (h *Handler) QueueRemoveJob(c echo.Context) error { return c.NoContent(http.StatusNoContent) }

// QueueRetryJob handles POST /api/admin/queue/retry-job.
func (h *Handler) QueueRetryJob(c echo.Context) error { return c.NoContent(http.StatusNoContent) }

// QueueShowJob handles POST /api/admin/queue/show-job.
func (h *Handler) QueueShowJob(c echo.Context) error {
	return c.JSON(http.StatusNotFound, errResp("NOT_FOUND", "Not found.", "00000000-0000-0000-0000-000000000000"))
}

// QueueShowJobLogs handles POST /api/admin/queue/show-job-logs.
func (h *Handler) QueueShowJobLogs(c echo.Context) error { return c.JSON(http.StatusOK, []any{}) }

// QueueStats handles POST /api/admin/queue/stats.
func (h *Handler) QueueStats(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{
		"deliver": map[string]any{"activeSince": nil, "active": 0, "waiting": 0, "delayed": 0},
		"inbox":   map[string]any{"activeSince": nil, "active": 0, "waiting": 0, "delayed": 0},
	})
}

// --- relays ---

// RelaysAdd handles POST /api/admin/relays/add.
func (h *Handler) RelaysAdd(c echo.Context) error {
	if h.adminDB == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		Inbox string `json:"inbox"`
	}
	_ = c.Bind(&req)
	if req.Inbox == "" {
		return c.NoContent(http.StatusNoContent)
	}
	relay := &model.Relay{
		ID: h.idGen.Generate(time.Now()), Inbox: req.Inbox, Status: "requesting",
	}
	h.adminDB.Create(relay)
	return c.JSON(http.StatusOK, relay)
}

// RelaysList handles POST /api/admin/relays/list.
func (h *Handler) RelaysList(c echo.Context) error {
	if h.adminDB == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	var relays []*model.Relay
	h.adminDB.Order(`"id" DESC`).Find(&relays)
	return c.JSON(http.StatusOK, relays)
}

// RelaysRemove handles POST /api/admin/relays/remove.
func (h *Handler) RelaysRemove(c echo.Context) error {
	if h.adminDB == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		ID string `json:"id"`
	}
	_ = c.Bind(&req)
	h.adminDB.Where(`"id" = ?`, req.ID).Delete(&model.Relay{})
	return c.NoContent(http.StatusNoContent)
}

// --- system-webhook ---

// SystemWebhookCreate handles POST /api/admin/system-webhook/create.
func (h *Handler) SystemWebhookCreate(c echo.Context) error {
	if h.adminDB == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		Name   string   `json:"name"`
		URL    string   `json:"url"`
		Secret string   `json:"secret"`
		On     []string `json:"on"`
	}
	_ = c.Bind(&req)
	sw := &model.SystemWebhook{
		ID: h.idGen.Generate(time.Now()), Name: req.Name, URL: req.URL, Secret: req.Secret,
		On: req.On, IsActive: true, UpdatedAt: time.Now(),
	}
	h.adminDB.Create(sw)
	return c.JSON(http.StatusOK, sw)
}

// SystemWebhookDelete handles POST /api/admin/system-webhook/delete.
func (h *Handler) SystemWebhookDelete(c echo.Context) error {
	if h.adminDB == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		ID string `json:"id"`
	}
	_ = c.Bind(&req)
	h.adminDB.Where(`"id" = ?`, req.ID).Delete(&model.SystemWebhook{})
	return c.NoContent(http.StatusNoContent)
}

// SystemWebhookList handles POST /api/admin/system-webhook/list.
func (h *Handler) SystemWebhookList(c echo.Context) error {
	if h.adminDB == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	var webhooks []*model.SystemWebhook
	h.adminDB.Order(`"id" DESC`).Find(&webhooks)
	return c.JSON(http.StatusOK, webhooks)
}

// SystemWebhookShow handles POST /api/admin/system-webhook/show.
func (h *Handler) SystemWebhookShow(c echo.Context) error {
	if h.adminDB == nil {
		return c.JSON(http.StatusNotFound, errResp("NOT_FOUND", "Not found.", "00000000-0000-0000-0000-000000000000"))
	}
	var req struct {
		ID string `json:"id"`
	}
	_ = c.Bind(&req)
	var sw model.SystemWebhook
	if err := h.adminDB.Where(`"id" = ?`, req.ID).First(&sw).Error; err != nil {
		return c.JSON(http.StatusNotFound, errResp("NOT_FOUND", "Not found.", "00000000-0000-0000-0000-000000000000"))
	}
	return c.JSON(http.StatusOK, sw)
}

// SystemWebhookTest handles POST /api/admin/system-webhook/test.
func (h *Handler) SystemWebhookTest(c echo.Context) error { return c.NoContent(http.StatusNoContent) }

// SystemWebhookUpdate handles POST /api/admin/system-webhook/update.
func (h *Handler) SystemWebhookUpdate(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}
