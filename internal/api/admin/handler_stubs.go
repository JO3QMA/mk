package admin

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	"github.com/shiroha-a/mk/internal/core/serverstats"
	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/misc"
	"github.com/shiroha-a/mk/internal/misc/smtp"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/queue"
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
	h.scheduleAccountCascade(req.UserID)
	return c.NoContent(http.StatusNoContent)
}

// AccountsFindByEmail handles POST /api/admin/accounts/find-by-email.
// user_profile.email 列を検索して、紐づく user を返す。本家 Misskey の
// admin/accounts/find-by-email と同等。
func (h *Handler) AccountsFindByEmail(c echo.Context) error {
	var req struct {
		Email string `json:"email"`
	}
	if err := c.Bind(&req); err != nil || req.Email == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "email is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	profile, err := h.userRepo.FindProfileByEmail(req.Email)
	if err != nil {
		return c.JSON(http.StatusNotFound, apierr.Error("USER_NOT_FOUND", "User not found.", "a504947-b888-4a99-9f62-8c4a0f3a3dab"))
	}
	user, err := h.userRepo.FindByID(profile.UserID)
	if err != nil {
		return c.JSON(http.StatusNotFound, apierr.Error("USER_NOT_FOUND", "User not found.", "a504947-b888-4a99-9f62-8c4a0f3a3dab"))
	}
	// 他の admin エンドポイント (ShowUser 等) と同じ packAdminUser を通して
	// Misskey 本家互換のレスポンス整形をする。生 model.User を返すと
	// inbox / sharedInbox / usernameLower 等の内部フィールドが漏れ、
	// createdAt / roles / policies 等のフロントが期待するフィールドが欠落する。
	return c.JSON(http.StatusOK, h.packAdminUser(user, profile))
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
	h.scheduleAccountCascade(req.UserID)
	return c.NoContent(http.StatusNoContent)
}

// scheduleAccountCascade queues the background cascade deletion. Errors
// from the enqueuer are logged but never surfaced — the admin flag flip
// is the user-visible source of truth, so a failed enqueue only delays
// the cleanup until the next manual retry.
func (h *Handler) scheduleAccountCascade(userID string) {
	if h.deleteAccountEnqueuer == nil || userID == "" {
		return
	}
	if err := h.deleteAccountEnqueuer.EnqueueDeleteAccount(queue.DeleteAccountPayload{UserID: userID}); err != nil {
		slog.Warn("admin: enqueue delete-account failed", "userId", userID, "err", err)
	}
}

// DeleteAllFilesOfUser handles POST /api/admin/delete-all-files-of-a-user.
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

// ForwardAbuseUserReport handles POST /api/admin/forward-abuse-user-report.
//
// 対象ユーザーがリモートの場合、system actor 署名で origin インスタンス
// の inbox へ ActivityPub Flag を配送し、DB 側 forwarded=true も立てる。
// ローカル通報の場合は配送スキップで DB フラグのみ更新する。
func (h *Handler) ForwardAbuseUserReport(c echo.Context) error {
	var req struct {
		ReportID string `json:"reportId"`
	}
	_ = c.Bind(&req)
	if req.ReportID == "" {
		return c.NoContent(http.StatusNoContent)
	}
	if h.abuseForwarder != nil {
		if err := h.abuseForwarder.ForwardReport(req.ReportID); err != nil {
			return apierr.JSONInternalError(c)
		}
		return c.NoContent(http.StatusNoContent)
	}
	// forwarder 未配線時のフォールバック: DB フラグだけ更新する (テストや
	// federation stack 未初期化パスで有効)。
	if h.abuseRepo != nil {
		_ = h.abuseRepo.UpdateFields(req.ReportID, map[string]any{"forwarded": true})
	}
	return c.NoContent(http.StatusNoContent)
}

// GetIndexStats handles POST /api/admin/get-index-stats.
//
// Returns per-index row counts from pg_stat_user_indexes so the admin UI can
// spot hot or unused indexes.
func (h *Handler) GetIndexStats(c echo.Context) error {
	if h.adminDB == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	type row struct {
		Relname      string `json:"tablename" gorm:"column:relname"`
		Indexrelname string `json:"indexname" gorm:"column:indexrelname"`
		IdxScan      int64  `json:"idx_scan" gorm:"column:idx_scan"`
		IdxTupRead   int64  `json:"idx_tup_read" gorm:"column:idx_tup_read"`
	}
	var rows []row
	if err := h.adminDB.Raw(`
		SELECT relname, indexrelname, idx_scan, idx_tup_read
		FROM pg_stat_user_indexes
		ORDER BY relname, indexrelname
	`).Scan(&rows).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	return c.JSON(http.StatusOK, rows)
}

// GetTableStats handles POST /api/admin/get-table-stats.
//
// Returns per-table size and row estimate via pg_stat_user_tables joined with
// pg_relation_size for quick capacity planning.
func (h *Handler) GetTableStats(c echo.Context) error {
	if h.adminDB == nil {
		return c.JSON(http.StatusOK, map[string]any{})
	}
	type row struct {
		Relname  string `gorm:"column:relname"`
		Count    int64  `gorm:"column:row_count"`
		SizeBase int64  `gorm:"column:size_base"`
		SizeIdx  int64  `gorm:"column:size_idx"`
	}
	var rows []row
	if err := h.adminDB.Raw(`
		SELECT c.relname,
		       COALESCE(s.n_live_tup, 0) AS row_count,
		       pg_relation_size(c.oid) AS size_base,
		       COALESCE(pg_indexes_size(c.oid), 0) AS size_idx
		FROM pg_class c
		LEFT JOIN pg_stat_user_tables s ON s.relid = c.oid
		WHERE c.relkind = 'r'
		  AND c.relnamespace IN (SELECT oid FROM pg_namespace WHERE nspname = 'public')
		ORDER BY c.relname
	`).Scan(&rows).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	// Misskey 本家は { tableName: { count, size } } の map 形式で返すのでそれに合わせる。
	result := make(map[string]any, len(rows))
	for _, r := range rows {
		result[r.Relname] = map[string]any{
			"count": r.Count,
			"size":  r.SizeBase + r.SizeIdx,
		}
	}
	return c.JSON(http.StatusOK, result)
}

// GetUserIPs handles POST /api/admin/get-user-ips.
func (h *Handler) GetUserIPs(c echo.Context) error {
	if h.userIPRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	var req struct {
		UserID string `json:"userId"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "userId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	ips, err := h.userIPRepo.ListByUser(req.UserID, 30)
	if err != nil {
		return c.JSON(http.StatusOK, []any{})
	}
	result := make([]map[string]any, 0, len(ips))
	for _, ip := range ips {
		result = append(result, map[string]any{
			"ip":        ip.IP,
			"createdAt": ip.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
		})
	}
	return c.JSON(http.StatusOK, result)
}

// ResetPassword handles POST /api/admin/reset-password.
//
// 優先: 対象ユーザーに verified email があれば reset token を発行してメール
// リンクを送り、HTTP 200 で {"sent": true} を返す (ユーザーが自分で新パス
// ワードを設定する本家互換フロー #186)。
//
// Fallback: email が verify されていない、repo / sender が未配線、
// etc. の場合は従来どおりランダムなテンポラリパスワードを生成してその場で
// 更新し、{"password": "..."} を返す。旧管理 UI との互換を保つ最後の砦。
func (h *Handler) ResetPassword(c echo.Context) error {
	var req struct {
		UserID string `json:"userId"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "userId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if sent := h.sendPasswordResetEmail(req.UserID); sent {
		return c.JSON(http.StatusOK, map[string]any{"sent": true})
	}
	return h.issueTemporaryPassword(c, req.UserID)
}

// sendPasswordResetEmail attempts the token+email flow. Returns true only
// when a reset token was persisted and the email handed off to the sender.
// Any missing dependency / unverified email triggers false for the caller
// to pick up the legacy path.
func (h *Handler) sendPasswordResetEmail(userID string) bool {
	if h.resetReqRepo == nil || h.emailSender == nil || h.userRepo == nil {
		return false
	}
	profile, err := h.userRepo.FindProfileByUserID(userID)
	if err != nil || profile.Email == nil || *profile.Email == "" || !profile.EmailVerified {
		return false
	}
	token := misc.SecureRandomHex(64)
	now := time.Now()
	resetReq := &model.PasswordResetRequest{
		ID:     h.idGen.Generate(now),
		Token:  token,
		UserID: userID,
	}
	if err := h.resetReqRepo.Create(resetReq); err != nil {
		return false
	}
	link := h.serverURL + "/reset-password/" + token
	go h.emailSender(*profile.Email, "Password reset",
		"An administrator initiated a password reset for your account.\n"+
			"Use the following link within 30 minutes to set a new password:\n"+link)
	return true
}

// issueTemporaryPassword is the legacy path retained for installations where
// email is not configured or the user has no verified address. Generates a
// random password, updates the profile, and returns it to the admin.
func (h *Handler) issueTemporaryPassword(c echo.Context, userID string) error {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	newPass := hex.EncodeToString(b)
	hash, _ := bcrypt.GenerateFromPassword([]byte(newPass), bcrypt.DefaultCost)
	if h.userRepo != nil {
		_ = h.userRepo.UpdateProfile(userID, map[string]any{"password": string(hash)})
	}
	return c.JSON(http.StatusOK, map[string]any{"password": newPass})
}

// SendEmail handles POST /api/admin/send-email.
func (h *Handler) SendEmail(c echo.Context) error {
	var req struct {
		To      string `json:"to"`
		Subject string `json:"subject"`
		Text    string `json:"text"`
	}
	if err := c.Bind(&req); err != nil || req.To == "" {
		return c.NoContent(http.StatusNoContent)
	}
	// SMTP送信
	if h.metaRepo != nil {
		m, err := h.metaRepo.Fetch()
		if err == nil && m.EnableEmail && m.SmtpHost != nil && m.Email != nil {
			port := 587
			if m.SmtpPort != nil {
				port = *m.SmtpPort
			}
			go smtp.Send(*m.SmtpHost, port, m.SmtpUser, m.SmtpPass, *m.Email, req.To, req.Subject, req.Text)
		}
	}
	return c.NoContent(http.StatusNoContent)
}

// ServerInfo handles POST /api/admin/server-info.
// meta.enableServerMachineStats が false ならゼロ値を返す。
func (h *Handler) ServerInfo(c echo.Context) error {
	if m, err := h.metaRepo.Fetch(); err == nil && m.EnableServerMachineStats {
		return c.JSON(http.StatusOK, serverstats.Collect())
	}
	return c.JSON(http.StatusOK, serverstats.Empty())
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
	var req struct {
		ReportID string `json:"reportId"`
	}
	_ = c.Bind(&req)
	if req.ReportID != "" && h.abuseRepo != nil {
		_ = h.abuseRepo.UpdateFields(req.ReportID, map[string]any{"resolved": true})
	}
	return c.NoContent(http.StatusNoContent)
}

// UpdateProxyAccount handles POST /api/admin/update-proxy-account.
//
// username (local) を解決してその user.id を meta.proxyAccountId に保存する。
// 空 / null を渡した場合は proxyAccountId を NULL に戻して proxy を無効化する。
func (h *Handler) UpdateProxyAccount(c echo.Context) error {
	var req struct {
		Username *string `json:"username"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "Invalid parameters.", "00000000-0000-0000-0000-000000000000"))
	}
	if h.metaRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	// 明示的な空文字 / null → 解除
	if req.Username == nil || *req.Username == "" {
		if err := h.metaRepo.Update(map[string]any{"proxyAccountId": nil}); err != nil {
			return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
		}
		return c.NoContent(http.StatusNoContent)
	}
	if h.userRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	user, err := h.userRepo.FindByUsernameLower(strings.ToLower(*req.Username), nil)
	if err != nil {
		return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_USER", "No such user.", "00000000-0000-0000-0000-000000000000"))
	}
	if err := h.metaRepo.Update(map[string]any{"proxyAccountId": user.ID}); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
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
	// 単一 DELETE 文なので同期実行で十分。将来バッチ化が必要ならここを
	// queue ジョブに差し替える。
	if h.driveFileRepo != nil {
		_, _ = h.driveFileRepo.DeleteRemoteCache()
	}
	return c.NoContent(http.StatusNoContent)
}

// DriveCleanup handles POST /api/admin/drive/cleanup.
func (h *Handler) DriveCleanup(c echo.Context) error {
	if h.driveFileRepo != nil {
		_, _ = h.driveFileRepo.DeleteOrphans()
	}
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
		Origin  string `json:"origin"`
		Host    string `json:"host"`
		Type    string `json:"type"`
	}
	_ = c.Bind(&req)
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 30
	}
	switch req.Origin {
	case "", "combined", "local", "remote":
	default:
		req.Origin = "combined"
	}
	files, err := h.driveFileRepo.ListForAdmin(req.Origin, req.Host, req.Type, req.UntilID, req.SinceID, req.Limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	out := make([]entity.DriveFileEntity, 0, len(files))
	for _, f := range files {
		out = append(out, h.packDriveFileAdmin(f))
	}
	return c.JSON(http.StatusOK, out)
}

// packDriveFileAdmin packs a drive file and embeds the owning user as
// nested `user` when userRepo is wired. Folder is left nil because the
// admin handler does not currently wire a DriveFolderRepository.
func (h *Handler) packDriveFileAdmin(f *model.DriveFile) entity.DriveFileEntity {
	// user repoが未設定なら user は nil fallback。folder は admin handler
	// に folderRepoがないため常に nil (必要になれば拡張)。
	var user *model.User
	if h.userRepo != nil && f.UserID != nil {
		if u, err := h.userRepo.FindByID(*f.UserID); err == nil {
			user = u
		}
	}
	return entity.PackDriveFileWithRelations(f, h.idGen, nil, user)
}

// DriveShowFile handles POST /api/admin/drive/show-file.
// Accepts either a fileId or a url as identifier (Misskey 本家互換)。
func (h *Handler) DriveShowFile(c echo.Context) error {
	var req struct {
		FileID string `json:"fileId"`
		URL    string `json:"url"`
	}
	if err := c.Bind(&req); err != nil || (req.FileID == "" && req.URL == "") {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "fileId or url is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if h.driveFileRepo == nil {
		return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_FILE", "No such file.", "ac4f7b11-1a6e-47e3-bf3d-3dce9a0e07ab"))
	}
	if req.FileID != "" {
		file, err := h.driveFileRepo.FindByID(req.FileID)
		if err != nil {
			return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_FILE", "No such file.", "ac4f7b11-1a6e-47e3-bf3d-3dce9a0e07ab"))
		}
		return c.JSON(http.StatusOK, h.packDriveFileAdmin(file))
	}
	// url 指定時は adminDB を使って url / thumbnailUrl / webpublicUrl いずれか
	// に一致する 1 件を引く。 driveFileRepo には該当 API が無いため raw query で。
	if h.adminDB == nil {
		return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_FILE", "No such file.", "ac4f7b11-1a6e-47e3-bf3d-3dce9a0e07ab"))
	}
	var file model.DriveFile
	if err := h.adminDB.Where(
		`"url" = ? OR "thumbnailUrl" = ? OR "webpublicUrl" = ?`,
		req.URL, req.URL, req.URL,
	).First(&file).Error; err != nil {
		return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_FILE", "No such file.", "ac4f7b11-1a6e-47e3-bf3d-3dce9a0e07ab"))
	}
	return c.JSON(http.StatusOK, h.packDriveFileAdmin(&file))
}

// --- emoji bulk ops ---

// EmojiAddAliasesBulk handles POST /api/admin/emoji/add-aliases-bulk.
//
// 旧実装は id ごとに FindByID→UpdateFields を直列に呼んでいたが、件数が多いと
// DB 往復が線形に増える。FindManyByIDs で一括取得 → 個別に alias を merge →
// UpdateFields で書き戻す (alias はレコード毎に異なるため UpdateFieldsMany
// では一括化できない)。
func (h *Handler) EmojiAddAliasesBulk(c echo.Context) error {
	var req struct {
		IDs     []string `json:"ids"`
		Aliases []string `json:"aliases"`
	}
	if err := c.Bind(&req); err != nil || len(req.IDs) == 0 {
		return c.NoContent(http.StatusNoContent)
	}
	if h.emojiRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	rows, err := h.emojiRepo.FindManyByIDs(req.IDs)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	for _, e := range rows {
		merged := dedupe(append(append([]string{}, e.Aliases...), req.Aliases...))
		_ = h.emojiRepo.UpdateFields(e.ID, map[string]any{"aliases": merged})
	}
	return c.NoContent(http.StatusNoContent)
}

// EmojiCopy handles POST /api/admin/emoji/copy.
//
// Misskey 本家は対象 name が既存ならば DUPLICATE_NAME を返す。サフィックス
// _copy を付けても既存に衝突する可能性があるため、明示チェックを入れる。
func (h *Handler) EmojiCopy(c echo.Context) error {
	var req struct {
		EmojiID string `json:"emojiId"`
	}
	if err := c.Bind(&req); err != nil || req.EmojiID == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "emojiId is required.", "00000000-0000-0000-0000-000000000000"))
	}
	if h.emojiRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	src, err := h.emojiRepo.FindByID(req.EmojiID)
	if err != nil {
		return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_EMOJI", "No such emoji.", "e2785b66-dca3-4087-9cac-b93c541cc425"))
	}
	newName := src.Name + "_copy"
	if existing, err := h.emojiRepo.FindByNameAndHost(newName, nil); err == nil && existing != nil {
		return c.JSON(http.StatusBadRequest, apierr.Error("DUPLICATE_NAME", "Duplicate name.", "5abef7f4-b3d6-4be4-a08f-4c4b7cf4f5b0"))
	}
	copied := *src
	copied.ID = h.idGen.Generate(time.Now())
	copied.Name = newName
	copied.Host = nil
	if err := h.emojiRepo.Create(&copied); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	return c.JSON(http.StatusOK, map[string]any{"id": copied.ID})
}

// EmojiDeleteBulk handles POST /api/admin/emoji/delete-bulk.
func (h *Handler) EmojiDeleteBulk(c echo.Context) error {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := c.Bind(&req); err != nil || len(req.IDs) == 0 {
		return c.NoContent(http.StatusNoContent)
	}
	if h.emojiRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	if err := h.emojiRepo.DeleteMany(req.IDs); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	return c.NoContent(http.StatusNoContent)
}

// dedupe returns the slice with duplicates removed while preserving order.
func dedupe(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// EmojiImportZip handles POST /api/admin/emoji/import-zip.
//
// fileId で Drive にアップロード済みの ZIP を指定し、非同期ジョブで展開して
// ローカルカスタム絵文字として登録する。本家 Misskey 互換
// (QueueService.createImportCustomEmojisJob 相当)。
func (h *Handler) EmojiImportZip(c echo.Context) error {
	var req struct {
		FileID string `json:"fileId"`
	}
	if err := c.Bind(&req); err != nil || req.FileID == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "fileId is required.", "5f4c9d8a-7c39-4bfa-9dcb-09f17e0f7a25"))
	}
	// drive file の存在確認 (ジョブが後で失敗するよりここで早期に弾く)
	if h.driveFileRepo != nil {
		if _, err := h.driveFileRepo.FindByID(req.FileID); err != nil {
			return c.JSON(http.StatusBadRequest, apierr.Error("NO_SUCH_FILE", "No such file.", "ac4f7b11-1a6e-47e3-bf3d-3dce9a0e07ab"))
		}
	}
	if h.emojiEnqueuer == nil {
		return c.NoContent(http.StatusNoContent)
	}
	user := middleware.GetUser(c)
	if user == nil {
		return c.NoContent(http.StatusNoContent)
	}
	if err := h.emojiEnqueuer.EnqueueImportCustomEmojis(queue.ImportCustomEmojisPayload{
		UserID: user.ID,
		FileID: req.FileID,
	}); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Failed to enqueue emoji import.", "89a6d9fd-0fe6-4c3c-9daa-7c6b1f29f1a4"))
	}
	return c.NoContent(http.StatusNoContent)
}

// EmojiListRemote handles POST /api/admin/emoji/list-remote.
func (h *Handler) EmojiListRemote(c echo.Context) error {
	if h.emojiRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	var req struct {
		Query  string `json:"query"`
		Host   string `json:"host"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	}
	_ = c.Bind(&req)
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 30
	}
	emojis, err := h.emojiRepo.ListRemoteWithFilter(req.Query, req.Host, req.Limit, req.Offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	if emojis == nil {
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
	if h.emojiRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	rows, err := h.emojiRepo.FindManyByIDs(req.IDs)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	removeSet := make(map[string]bool, len(req.Aliases))
	for _, a := range req.Aliases {
		removeSet[a] = true
	}
	for _, e := range rows {
		filtered := make([]string, 0, len(e.Aliases))
		for _, a := range e.Aliases {
			if !removeSet[a] {
				filtered = append(filtered, a)
			}
		}
		_ = h.emojiRepo.UpdateFields(e.ID, map[string]any{"aliases": filtered})
	}
	return c.NoContent(http.StatusNoContent)
}

// EmojiSetAliasesBulk handles POST /api/admin/emoji/set-aliases-bulk.
//
// 全 id で同じ aliases 値を設定するため UpdateFieldsMany (WHERE IN UPDATE) で
// 1 文に集約できる。
func (h *Handler) EmojiSetAliasesBulk(c echo.Context) error {
	var req struct {
		IDs     []string `json:"ids"`
		Aliases []string `json:"aliases"`
	}
	if err := c.Bind(&req); err != nil || len(req.IDs) == 0 {
		return c.NoContent(http.StatusNoContent)
	}
	if h.emojiRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	if err := h.emojiRepo.UpdateFieldsMany(req.IDs, map[string]any{"aliases": req.Aliases}); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
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
	if h.emojiRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	if err := h.emojiRepo.UpdateFieldsMany(req.IDs, map[string]any{"category": req.Category}); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
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
	if h.emojiRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	if err := h.emojiRepo.UpdateFieldsMany(req.IDs, map[string]any{"license": req.License}); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	return c.NoContent(http.StatusNoContent)
}

// --- captcha ---

// CaptchaCurrent handles POST /api/admin/captcha/current.
//
// Returns which captcha provider (if any) is currently enabled and its public
// site key so the admin UI can render the correct configuration form.
func (h *Handler) CaptchaCurrent(c echo.Context) error {
	if h.metaRepo == nil {
		return c.JSON(http.StatusOK, map[string]any{"provider": nil})
	}
	meta, err := h.metaRepo.Fetch()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	var provider any
	var siteKey *string
	switch {
	case meta.EnableHcaptcha:
		provider = "hcaptcha"
		siteKey = meta.HcaptchaSiteKey
	case meta.EnableRecaptcha:
		provider = "recaptcha"
		siteKey = meta.RecaptchaSiteKey
	case meta.EnableTurnstile:
		provider = "turnstile"
		siteKey = meta.TurnstileSiteKey
	}
	return c.JSON(http.StatusOK, map[string]any{
		"provider": provider,
		"siteKey":  siteKey,
	})
}

// CaptchaSave handles POST /api/admin/captcha/save.
func (h *Handler) CaptchaSave(c echo.Context) error {
	var req struct {
		Provider    string  `json:"provider"`
		HcaptchaSK  *string `json:"hcaptchaSiteKey"`
		HcaptchaSS  *string `json:"hcaptchaSecretKey"`
		RecaptchaSK *string `json:"recaptchaSiteKey"`
		RecaptchaSS *string `json:"recaptchaSecretKey"`
		TurnstileSK *string `json:"turnstileSiteKey"`
		TurnstileSS *string `json:"turnstileSecretKey"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "Invalid parameters.", "00000000-0000-0000-0000-000000000000"))
	}
	// provider が空 or "none" の場合は captcha 無効化として扱う。
	switch req.Provider {
	case "", "none", "hcaptcha", "recaptcha", "turnstile":
	default:
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "Unknown captcha provider.", "00000000-0000-0000-0000-000000000000"))
	}
	if h.metaRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	fields := map[string]any{
		"enableHcaptcha":  req.Provider == "hcaptcha",
		"enableRecaptcha": req.Provider == "recaptcha",
		"enableTurnstile": req.Provider == "turnstile",
	}
	if req.HcaptchaSK != nil {
		fields["hcaptchaSiteKey"] = *req.HcaptchaSK
	}
	if req.HcaptchaSS != nil {
		fields["hcaptchaSecretKey"] = *req.HcaptchaSS
	}
	if req.RecaptchaSK != nil {
		fields["recaptchaSiteKey"] = *req.RecaptchaSK
	}
	if req.RecaptchaSS != nil {
		fields["recaptchaSecretKey"] = *req.RecaptchaSS
	}
	if req.TurnstileSK != nil {
		fields["turnstileSiteKey"] = *req.TurnstileSK
	}
	if req.TurnstileSS != nil {
		fields["turnstileSecretKey"] = *req.TurnstileSS
	}
	if err := h.metaRepo.Update(fields); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	return c.NoContent(http.StatusNoContent)
}

// --- ad ---

// AdCreate handles POST /api/admin/ad/create.
func (h *Handler) AdCreate(c echo.Context) error {
	if h.adRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		URL         string `json:"url"`
		ImageURL    string `json:"imageUrl"`
		Place       string `json:"place"`
		Memo        string `json:"memo"`
		Priority    string `json:"priority"`
		Ratio       int    `json:"ratio"`
		DayOfWeek   int    `json:"dayOfWeek"`
		ExpiresAt   int64  `json:"expiresAt"`
		StartsAt    int64  `json:"startsAt"`
		IsSensitive bool   `json:"isSensitive"`
	}
	if err := c.Bind(&req); err != nil || req.URL == "" || req.ImageURL == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "Invalid parameters.", "00000000-0000-0000-0000-000000000000"))
	}
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
		ID:          h.idGen.Generate(time.Now()),
		URL:         req.URL,
		ImageURL:    req.ImageURL,
		Place:       req.Place,
		Memo:        req.Memo,
		Priority:    req.Priority,
		Ratio:       req.Ratio,
		DayOfWeek:   req.DayOfWeek,
		IsSensitive: req.IsSensitive,
		StartsAt:    millisOrNow(req.StartsAt),
		ExpiresAt:   millisOrDefault(req.ExpiresAt, time.Now().Add(30*24*time.Hour)),
	}
	if err := h.adRepo.Create(ad); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	return c.JSON(http.StatusOK, ad)
}

// AdDelete handles POST /api/admin/ad/delete.
func (h *Handler) AdDelete(c echo.Context) error {
	if h.adRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		ID string `json:"id"`
	}
	_ = c.Bind(&req)
	_ = h.adRepo.Delete(req.ID)
	return c.NoContent(http.StatusNoContent)
}

// AdList handles POST /api/admin/ad/list.
func (h *Handler) AdList(c echo.Context) error {
	if h.adRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	var req struct {
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	}
	_ = c.Bind(&req)
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 30
	}
	rows, err := h.adRepo.List(req.Limit, req.Offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	if rows == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	return c.JSON(http.StatusOK, rows)
}

// AdUpdate handles POST /api/admin/ad/update.
func (h *Handler) AdUpdate(c echo.Context) error {
	// 旧実装が `c.Request().Body` を `Updates` に直接渡しており部分更新が機能して
	// いなかった。partial field map に差し替え、リクエストで明示された項目のみ
	// 書き換える。
	if h.adRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		ID          string  `json:"id"`
		URL         *string `json:"url"`
		ImageURL    *string `json:"imageUrl"`
		Place       *string `json:"place"`
		Memo        *string `json:"memo"`
		Priority    *string `json:"priority"`
		Ratio       *int    `json:"ratio"`
		DayOfWeek   *int    `json:"dayOfWeek"`
		ExpiresAt   *int64  `json:"expiresAt"`
		StartsAt    *int64  `json:"startsAt"`
		IsSensitive *bool   `json:"isSensitive"`
	}
	if err := c.Bind(&req); err != nil || req.ID == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "Invalid parameters.", "00000000-0000-0000-0000-000000000000"))
	}
	if _, err := h.adRepo.FindByID(req.ID); err != nil {
		return c.JSON(http.StatusNotFound, apierr.Error("NOT_FOUND", "Not found.", "00000000-0000-0000-0000-000000000000"))
	}
	fields := map[string]any{}
	if req.URL != nil {
		fields["url"] = *req.URL
	}
	if req.ImageURL != nil {
		fields["imageUrl"] = *req.ImageURL
	}
	if req.Place != nil {
		fields["place"] = *req.Place
	}
	if req.Memo != nil {
		fields["memo"] = *req.Memo
	}
	if req.Priority != nil {
		fields["priority"] = *req.Priority
	}
	if req.Ratio != nil {
		fields["ratio"] = *req.Ratio
	}
	if req.DayOfWeek != nil {
		fields["dayOfWeek"] = *req.DayOfWeek
	}
	if req.IsSensitive != nil {
		fields["isSensitive"] = *req.IsSensitive
	}
	// pointer 型フィールドは nil (未指定) と 0 (明示的に 0) を区別できるので、
	// 受け取った値をそのまま UnixMilli に変換する。Create 側と違い 0 を now に
	// 読み替えない。
	if req.StartsAt != nil {
		fields["startsAt"] = time.UnixMilli(*req.StartsAt)
	}
	if req.ExpiresAt != nil {
		fields["expiresAt"] = time.UnixMilli(*req.ExpiresAt)
	}
	if err := h.adRepo.UpdateFields(req.ID, fields); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	return c.NoContent(http.StatusNoContent)
}

// millisOrNow converts a UNIX millisecond timestamp to time.Time; 0 falls back
// to the current wall clock.
func millisOrNow(ms int64) time.Time {
	if ms == 0 {
		return time.Now()
	}
	return time.UnixMilli(ms)
}

// millisOrDefault converts a UNIX millisecond timestamp to time.Time; 0 falls
// back to the provided default.
func millisOrDefault(ms int64, def time.Time) time.Time {
	if ms == 0 {
		return def
	}
	return time.UnixMilli(ms)
}

// --- avatar-decorations ---

// AvatarDecorationsCreate handles POST /api/admin/avatar-decorations/create.
func (h *Handler) AvatarDecorationsCreate(c echo.Context) error {
	if h.avatarDecoRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		URL         string   `json:"url"`
		RoleIDs     []string `json:"roleIdsThatCanBeUsedThisDecoration"`
	}
	if err := c.Bind(&req); err != nil || req.Name == "" || req.URL == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "Invalid parameters.", "00000000-0000-0000-0000-000000000000"))
	}
	d := &model.AvatarDecoration{
		ID:          h.idGen.Generate(time.Now()),
		Name:        req.Name,
		Description: req.Description,
		URL:         req.URL,
		RoleIDs:     req.RoleIDs,
	}
	if err := h.avatarDecoRepo.Create(d); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	return c.JSON(http.StatusOK, d)
}

// AvatarDecorationsDelete handles POST /api/admin/avatar-decorations/delete.
func (h *Handler) AvatarDecorationsDelete(c echo.Context) error {
	if h.avatarDecoRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		ID string `json:"id"`
	}
	_ = c.Bind(&req)
	_ = h.avatarDecoRepo.Delete(req.ID)
	return c.NoContent(http.StatusNoContent)
}

// AvatarDecorationsList handles POST /api/admin/avatar-decorations/list.
func (h *Handler) AvatarDecorationsList(c echo.Context) error {
	if h.avatarDecoRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	rows, err := h.avatarDecoRepo.List()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	if rows == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	return c.JSON(http.StatusOK, rows)
}

// AvatarDecorationsUpdate handles POST /api/admin/avatar-decorations/update.
func (h *Handler) AvatarDecorationsUpdate(c echo.Context) error {
	if h.avatarDecoRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		ID          string    `json:"id"`
		Name        *string   `json:"name"`
		Description *string   `json:"description"`
		URL         *string   `json:"url"`
		RoleIDs     *[]string `json:"roleIdsThatCanBeUsedThisDecoration"`
	}
	if err := c.Bind(&req); err != nil || req.ID == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "Invalid parameters.", "00000000-0000-0000-0000-000000000000"))
	}
	if _, err := h.avatarDecoRepo.FindByID(req.ID); err != nil {
		return c.JSON(http.StatusNotFound, apierr.Error("NOT_FOUND", "Not found.", "00000000-0000-0000-0000-000000000000"))
	}
	fields := map[string]any{"updatedAt": time.Now()}
	if req.Name != nil {
		fields["name"] = *req.Name
	}
	if req.Description != nil {
		fields["description"] = *req.Description
	}
	if req.URL != nil {
		fields["url"] = *req.URL
	}
	if req.RoleIDs != nil {
		fields["roleIdsThatCanBeUsedThisDecoration"] = *req.RoleIDs
	}
	if err := h.avatarDecoRepo.UpdateFields(req.ID, fields); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	return c.NoContent(http.StatusNoContent)
}

// --- abuse-report/notification-recipient ---

// AbuseReportNotificationRecipientCreate handles POST /api/admin/abuse-report/notification-recipient/create.
func (h *Handler) AbuseReportNotificationRecipientCreate(c echo.Context) error {
	if h.recipientRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		Name            string  `json:"name"`
		Method          string  `json:"method"`
		UserID          *string `json:"userId"`
		SystemWebhookID *string `json:"systemWebhookId"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "Invalid parameters.", "00000000-0000-0000-0000-000000000000"))
	}
	if req.Method == "" {
		req.Method = "email"
	}
	r := &model.AbuseReportNotificationRecipient{
		ID:              h.idGen.Generate(time.Now()),
		Name:            req.Name,
		Method:          req.Method,
		UserID:          req.UserID,
		SystemWebhookID: req.SystemWebhookID,
		IsActive:        true,
	}
	if err := h.recipientRepo.Create(r); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	return c.JSON(http.StatusOK, r)
}

// AbuseReportNotificationRecipientDelete handles POST /api/admin/abuse-report/notification-recipient/delete.
func (h *Handler) AbuseReportNotificationRecipientDelete(c echo.Context) error {
	if h.recipientRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		ID string `json:"id"`
	}
	_ = c.Bind(&req)
	_ = h.recipientRepo.Delete(req.ID)
	return c.NoContent(http.StatusNoContent)
}

// AbuseReportNotificationRecipientList handles POST /api/admin/abuse-report/notification-recipient/list.
func (h *Handler) AbuseReportNotificationRecipientList(c echo.Context) error {
	if h.recipientRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	rows, err := h.recipientRepo.List()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	// nil を明示的に空配列化 (クライアント互換)
	if rows == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	return c.JSON(http.StatusOK, rows)
}

// AbuseReportNotificationRecipientShow handles POST /api/admin/abuse-report/notification-recipient/show.
func (h *Handler) AbuseReportNotificationRecipientShow(c echo.Context) error {
	if h.recipientRepo == nil {
		return c.JSON(http.StatusNotFound, apierr.Error("NOT_FOUND", "Not found.", "00000000-0000-0000-0000-000000000000"))
	}
	var req struct {
		ID string `json:"id"`
	}
	_ = c.Bind(&req)
	r, err := h.recipientRepo.FindByID(req.ID)
	if err != nil {
		return c.JSON(http.StatusNotFound, apierr.Error("NOT_FOUND", "Not found.", "00000000-0000-0000-0000-000000000000"))
	}
	return c.JSON(http.StatusOK, r)
}

// AbuseReportNotificationRecipientUpdate handles POST /api/admin/abuse-report/notification-recipient/update.
func (h *Handler) AbuseReportNotificationRecipientUpdate(c echo.Context) error {
	if h.recipientRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		ID              string  `json:"id"`
		Name            *string `json:"name"`
		Method          *string `json:"method"`
		IsActive        *bool   `json:"isActive"`
		UserID          *string `json:"userId"`
		SystemWebhookID *string `json:"systemWebhookId"`
	}
	if err := c.Bind(&req); err != nil || req.ID == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "Invalid parameters.", "00000000-0000-0000-0000-000000000000"))
	}
	fields := map[string]any{}
	if req.Name != nil {
		fields["name"] = *req.Name
	}
	if req.Method != nil {
		fields["method"] = *req.Method
	}
	if req.IsActive != nil {
		fields["isActive"] = *req.IsActive
	}
	if req.UserID != nil {
		fields["userId"] = *req.UserID
	}
	if req.SystemWebhookID != nil {
		fields["systemWebhookId"] = *req.SystemWebhookID
	}
	// GORM Updates(map) は対象なしでも nil を返すため、ここでのエラーは DB 障害
	// 等の真の失敗。NotFound は続く FindByID で検出する。
	if err := h.recipientRepo.Update(req.ID, fields); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	r, err := h.recipientRepo.FindByID(req.ID)
	if err != nil {
		return c.JSON(http.StatusNotFound, apierr.Error("NOT_FOUND", "Not found.", "00000000-0000-0000-0000-000000000000"))
	}
	return c.JSON(http.StatusOK, r)
}

// --- federation ---

// FederationDeleteAllFiles handles POST /api/admin/federation/delete-all-files.
func (h *Handler) FederationDeleteAllFiles(c echo.Context) error {
	var req struct {
		Host string `json:"host"`
	}
	_ = c.Bind(&req)
	// リモートファイル削除は将来対応 (DriveFileRepoのリモートファイル一括削除が必要)
	return c.NoContent(http.StatusNoContent)
}

// FederationRefreshRemoteInstanceMetadata handles POST /api/admin/federation/refresh-remote-instance-metadata.
func (h *Handler) FederationRefreshRemoteInstanceMetadata(c echo.Context) error {
	var req struct {
		Host string `json:"host"`
	}
	_ = c.Bind(&req)
	// メタデータ再取得は将来対応 (FetchMetadataServiceの呼び出しが必要)
	return c.NoContent(http.StatusNoContent)
}

// FederationRemoveAllFollowing handles POST /api/admin/federation/remove-all-following.
func (h *Handler) FederationRemoveAllFollowing(c echo.Context) error {
	var req struct {
		Host string `json:"host"`
	}
	_ = c.Bind(&req)
	// リモートフォロー一括削除は将来対応
	return c.NoContent(http.StatusNoContent)
}

// FederationUpdateInstance handles POST /api/admin/federation/update-instance.
func (h *Handler) FederationUpdateInstance(c echo.Context) error {
	if h.adminDB == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		Host           string `json:"host"`
		IsSuspended    *bool  `json:"isSuspended"`
		IsBlocked      *bool  `json:"isBlocked"`
		IsSilenced     *bool  `json:"isSilenced"`
		ModerationNote string `json:"moderationNote"`
	}
	if err := c.Bind(&req); err != nil || req.Host == "" {
		return c.NoContent(http.StatusNoContent)
	}
	updates := map[string]any{}
	if req.IsSuspended != nil {
		updates["isSuspended"] = *req.IsSuspended
	}
	if req.IsBlocked != nil {
		updates["isBlocked"] = *req.IsBlocked
	}
	if req.IsSilenced != nil {
		updates["isSilenced"] = *req.IsSilenced
	}
	if req.ModerationNote != "" {
		updates["moderationNote"] = req.ModerationNote
	}
	if len(updates) > 0 {
		h.adminDB.Model(&model.Instance{}).Where(`"host" = ?`, req.Host).Updates(updates)
	}
	return c.NoContent(http.StatusNoContent)
}

// --- invite ---

// InviteCreate handles POST /api/admin/invite/create.
func (h *Handler) InviteCreate(c echo.Context) error {
	// 本家 TS は count (1-100, default 1) 分のチケットを配列で返す。個々の Create
	// 失敗時でも既作成分はロールバックしない (本家も Promise.all で非原子的)。
	if h.inviteRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		Count     int     `json:"count"`
		ExpiresAt *string `json:"expiresAt"`
	}
	_ = c.Bind(&req)
	if req.Count <= 0 {
		req.Count = 1
	}
	if req.Count > 100 {
		req.Count = 100
	}
	var expiresAt *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		parsed, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_DATE_TIME", "Invalid date-time format", "f1380b15-3760-4c6c-a1db-5c3aaf1cbd49"))
		}
		expiresAt = &parsed
	}
	user := middleware.GetUser(c)
	var createdByID *string
	if user != nil {
		createdByID = &user.ID
	}
	tickets := make([]*model.RegistrationTicket, 0, req.Count)
	for i := 0; i < req.Count; i++ {
		b := make([]byte, 16)
		_, _ = rand.Read(b)
		t := &model.RegistrationTicket{
			ID:          h.idGen.Generate(time.Now()),
			Code:        hex.EncodeToString(b),
			ExpiresAt:   expiresAt,
			CreatedByID: createdByID,
		}
		if err := h.inviteRepo.Create(t); err != nil {
			continue
		}
		tickets = append(tickets, t)
	}
	return c.JSON(http.StatusOK, h.packInviteTickets(tickets))
}

// packInviteTickets transforms RegistrationTicket rows into the
// Misskey-compatible InviteCodeEntityService.pack shape.
func (h *Handler) packInviteTickets(rows []*model.RegistrationTicket) []map[string]any {
	// Misskey 本家 InviteCodeEntityService.pack と同じ形にする。
	// createdAt は aidx ID から抽出、used は usedAt の有無で導出する。
	out := make([]map[string]any, 0, len(rows))
	for _, t := range rows {
		var createdAt *string
		if h.idGen != nil {
			if ts, err := h.idGen.ParseTime(t.ID); err == nil {
				s := ts.UTC().Format("2006-01-02T15:04:05.000Z")
				createdAt = &s
			}
		}
		var expiresAt *string
		if t.ExpiresAt != nil {
			s := t.ExpiresAt.UTC().Format("2006-01-02T15:04:05.000Z")
			expiresAt = &s
		}
		var usedAt *string
		if t.UsedAt != nil {
			s := t.UsedAt.UTC().Format("2006-01-02T15:04:05.000Z")
			usedAt = &s
		}
		out = append(out, map[string]any{
			"id":          t.ID,
			"code":        t.Code,
			"expiresAt":   expiresAt,
			"createdAt":   createdAt,
			"createdBy":   nil,
			"usedBy":      nil,
			"usedAt":      usedAt,
			"used":        t.UsedAt != nil,
			"createdById": t.CreatedByID,
			"usedById":    t.UsedByID,
		})
	}
	return out
}

// InviteList handles POST /api/admin/invite/list.
func (h *Handler) InviteList(c echo.Context) error {
	if h.inviteRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	var req struct {
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
		Type   string `json:"type"`
	}
	_ = c.Bind(&req)
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 30
	}
	filter := req.Type
	switch filter {
	case "unused", "used", "expired", "all":
	default:
		filter = "all"
	}
	rows, err := h.inviteRepo.List(filter, req.Limit, req.Offset, time.Now())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	return c.JSON(http.StatusOK, h.packInviteTickets(rows))
}

// --- promo ---

// PromoCreate handles POST /api/admin/promo/create.
func (h *Handler) PromoCreate(c echo.Context) error {
	if h.promoNoteRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		NoteID    string `json:"noteId"`
		ExpiresAt int64  `json:"expiresAt"`
	}
	if err := c.Bind(&req); err != nil || req.NoteID == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "noteId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}

	// 対象 note の存在確認 → 既に promote 済みでないか確認
	var targetUserID string
	if h.noteFinder != nil {
		note, err := h.noteFinder.FindByID(req.NoteID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, apierr.Error("NO_SUCH_NOTE", "No such note.", "ee449fbe-af2a-453b-9cae-cf2fe7c895fc"))
		}
		targetUserID = note.UserID
	}

	exists, err := h.promoNoteRepo.Exists(req.NoteID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	if exists {
		return c.JSON(http.StatusBadRequest, apierr.Error("ALREADY_PROMOTED", "The note has already promoted.", "ae427aa2-7a41-484f-a18c-2c1104051604"))
	}

	// note 情報が取れなかった場合は呼び出し admin を userId として記録 (最低限の整合)
	if targetUserID == "" {
		if u := middleware.GetUser(c); u != nil {
			targetUserID = u.ID
		}
	}

	promo := &model.PromoNote{
		NoteID:    req.NoteID,
		ExpiresAt: time.UnixMilli(req.ExpiresAt),
		UserID:    targetUserID,
	}
	if err := h.promoNoteRepo.Create(promo); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	return c.NoContent(http.StatusNoContent)
}

// --- queue ---

// QueueClear handles POST /api/admin/queue/clear.
func (h *Handler) QueueClear(c echo.Context) error {
	if h.queueInspector == nil {
		return c.NoContent(http.StatusNoContent)
	}
	queues, err := h.queueInspector.Queues()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	for _, q := range queues {
		_, _ = h.queueInspector.DeleteAllPendingTasks(q)
	}
	return c.NoContent(http.StatusNoContent)
}

// QueueDeliverDelayed handles POST /api/admin/queue/deliver-delayed.
// Returns scheduled/retry tasks on the `deliver` queue.
func (h *Handler) QueueDeliverDelayed(c echo.Context) error {
	return h.listDelayedTasks(c, "deliver")
}

// QueueInboxDelayed handles POST /api/admin/queue/inbox-delayed.
// Returns scheduled/retry tasks on the `inbox` queue.
func (h *Handler) QueueInboxDelayed(c echo.Context) error {
	return h.listDelayedTasks(c, "inbox")
}

// delayedTasksMaxFetch is the upper bound on how many of each of scheduled /
// retry we pull from asynq to build the virtual delayed list. asynq pages
// scheduled and retry independently so we cannot naively forward the user's
// page number to both (retry items before the scheduled boundary would
// disappear). We instead fetch the first asynq page (up to 100 items) of each,
// merge scheduled-first, then slice by the user's (page, limit).
//
// 想定: admin/queue/*-delayed は通常 "stuck な配送" を目視で確認する用途で、
// 合計 200 件を超えるケースは運用上ほぼ存在しない。深いページングが必要なら
// /admin/queue/jobs を state=scheduled / state=retry で使う。
const delayedTasksMaxFetch = 100

func (h *Handler) listDelayedTasks(c echo.Context, queueName string) error {
	if h.queueInspector == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	var req struct {
		Limit int `json:"limit"`
		Page  int `json:"page"`
	}
	_ = c.Bind(&req)
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 30
	}
	if req.Page < 1 {
		req.Page = 1
	}

	scheduled, _ := h.queueInspector.ListScheduledTasks(queueName, 1, delayedTasksMaxFetch)
	retry, _ := h.queueInspector.ListRetryTasks(queueName, 1, delayedTasksMaxFetch)
	combined := make([]*QueueTaskSummary, 0, len(scheduled)+len(retry))
	combined = append(combined, scheduled...)
	combined = append(combined, retry...)

	offset := (req.Page - 1) * req.Limit
	if offset >= len(combined) {
		return c.JSON(http.StatusOK, []map[string]any{})
	}
	end := offset + req.Limit
	if end > len(combined) {
		end = len(combined)
	}
	out := make([]map[string]any, 0, end-offset)
	for _, t := range combined[offset:end] {
		out = append(out, packTaskSummary(t))
	}
	return c.JSON(http.StatusOK, out)
}

// QueueJobs handles POST /api/admin/queue/jobs.
//
// frontend admin/job-queue.vue は state を Bull の state 名配列で送る
// (`['completed', 'failed', 'active', 'delayed', 'wait']` など)。
// mk-go は asynq バックなので Bull state 名を asynq の list 呼び出しに
// マッピングする。合計 limit を超えないよう走査中に切り詰める。
func (h *Handler) QueueJobs(c echo.Context) error {
	if h.queueInspector == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	// state は string でも string[] でも受け取れるようにする (frontend は
	// 配列、既存テストや admin CLI からは単一文字列でくる可能性がある)。
	var req struct {
		Queue  string          `json:"queue"`
		State  json.RawMessage `json:"state"`
		Limit  int             `json:"limit"`
		Page   int             `json:"page"`
		Search string          `json:"search"`
	}
	_ = c.Bind(&req)
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 30
	}
	if req.Page < 1 {
		req.Page = 1
	}
	states := parseStateField(req.State)
	queues := []string{req.Queue}
	if req.Queue == "" {
		qs, err := h.queueInspector.Queues()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
		}
		queues = qs
	}
	seen := make(map[string]struct{}, req.Limit)
	out := make([]map[string]any, 0, req.Limit)
outer:
	for _, q := range queues {
		for _, state := range states {
			rows, err := h.listTasksForState(q, state, req.Page, req.Limit)
			if err != nil {
				continue
			}
			for _, t := range rows {
				if len(out) >= req.Limit {
					break outer
				}
				// state指定が複数重なる (例: all タブ) とき同じ task が
				// 重複しないよう ID で de-dup する。
				if _, dup := seen[t.ID]; dup {
					continue
				}
				seen[t.ID] = struct{}{}
				out = append(out, packTaskSummary(t))
			}
		}
	}
	return c.JSON(http.StatusOK, out)
}

// parseStateField normalizes the `state` request field which can be a single
// string or an array of strings (Misskey frontend sends array). Empty input
// defaults to "wait" (Bull wording) = asynq "pending".
func parseStateField(raw json.RawMessage) []string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return []string{"wait"}
	}
	// try as array first
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		return arr
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil && single != "" {
		return []string{single}
	}
	return []string{"wait"}
}

// listTasksForState maps a Bull/asynq state name to the matching asynq list
// call. Returns an empty slice for states asynq does not support (completed /
// failed / paused) so the admin tab renders an empty list instead of 500.
func (h *Handler) listTasksForState(queue, state string, page, limit int) ([]*QueueTaskSummary, error) {
	switch state {
	case "active":
		return h.queueInspector.ListActiveTasks(queue, page, limit)
	case "scheduled":
		return h.queueInspector.ListScheduledTasks(queue, page, limit)
	case "retry":
		return h.queueInspector.ListRetryTasks(queue, page, limit)
	// Bull と asynq の用語対応
	case "wait", "pending":
		return h.queueInspector.ListPendingTasks(queue, page, limit)
	case "delayed":
		// delayed は Bull 用語で asynq の scheduled + retry に対応する。
		sched, _ := h.queueInspector.ListScheduledTasks(queue, page, limit)
		retry, _ := h.queueInspector.ListRetryTasks(queue, page, limit)
		return append(sched, retry...), nil
	case "completed", "failed", "paused":
		// asynq は retention 未設定だと completed/failed 履歴を保持しない。
		// 履歴APIが無いので空配列でフロントを通す (tab 表示自体は出す)。
		return nil, nil
	default:
		return h.queueInspector.ListPendingTasks(queue, page, limit)
	}
}

// QueuePromoteJobs handles POST /api/admin/queue/promote-jobs.
func (h *Handler) QueuePromoteJobs(c echo.Context) error {
	// asynq に bulk promote API が無いため、全キューの scheduled/retry を 1 ページ
	// 分ずつ拾って RunTask で逐次 promote する。大量投入時は後続のページを
	// クライアント側で再呼び出しする運用。
	if h.queueInspector == nil {
		return c.NoContent(http.StatusNoContent)
	}
	queues, err := h.queueInspector.Queues()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	promoted := 0
	for _, q := range queues {
		for _, state := range []string{"scheduled", "retry"} {
			var rows []*QueueTaskSummary
			if state == "scheduled" {
				rows, _ = h.queueInspector.ListScheduledTasks(q, 1, 100)
			} else {
				rows, _ = h.queueInspector.ListRetryTasks(q, 1, 100)
			}
			for _, t := range rows {
				if err := h.queueInspector.RunTask(q, t.ID); err == nil {
					promoted++
				}
			}
		}
	}
	return c.JSON(http.StatusOK, map[string]any{"promoted": promoted})
}

// shapeQueueForFrontend adapts a QueueInfoResult to the Misskey Bull-shaped
// JSON expected by the admin/job-queue.vue page.
//
// BullMQ (Misskey本家のjob queue) と asynq は設計思想が根本的に異なり、
// 完全互換は不可能：
//   - asynq に存在しない queue 名 (inbox / db / relationship / system 等):
//     frontend は Misskey.queueTypes で hardcode しており mk-go の queue 名
//     (deliver / push / webhook / export / maintenance) と一致しない。
//   - db.{memory,uptime,clients} は BullMQ が per-queue で持つ Redis 統計。
//     asynq には相当機能が無い。
//   - metrics.{completed,failed}.data は BullMQ の per-queue time-series。
//     asynq は retention 設定なしでは履歴を持たない。
//
// BullMQ と完全互換のまま Go ネイティブ性能を活かすのは別レイヤ (独立
// OSS ライブラリ化) で取り組む方針 (#377 参照)。それまでの中継措置として
// 見た目が壊れないよう未対応 field を 0 固定で stub する。
func shapeQueueForFrontend(info *QueueInfoResult) map[string]any {
	// delayed は Misskey 用語で scheduled + retry の合計に相当する
	// (どちらも「すぐには実行されない」状態)。
	delayed := info.Scheduled + info.Retry
	return map[string]any{
		"name":          info.Queue,
		"qualifiedName": info.Queue,
		"isPaused":      false,
		"counts": map[string]any{
			"active":    info.Active,
			"delayed":   delayed,
			"waiting":   info.Pending,
			"completed": info.Completed,
			"failed":    info.Failed,
		},
		"metrics": map[string]any{
			"completed": map[string]any{"data": []int{}, "count": info.Completed},
			"failed":    map[string]any{"data": []int{}, "count": info.Failed},
		},
		// asynq にはBull相当のper-queue redis DB statsがないため、frontend
		// が参照するフィールドを0固定でstubする。
		"db": map[string]any{
			"processId": 0,
			"port":      0,
			"runId":     "",
			"clients":   map[string]any{"connected": 0, "blocked": 0},
			"memory":    map[string]any{"peak": 0, "total": 0, "used": 0},
			"uptime":    0,
		},
	}
}

// QueueQueueStats handles POST /api/admin/queue/queue-stats.
// frontend admin/job-queue.vue の `fetchCurrentQueue` は単一 queue 名を
// 受けて 1 queue の shape を返す設計になっているので、req.Queue が来たら
// その queue だけ、来なければ全 queue を返すという両対応にする。
//
// frontend は Misskey Bull の queue 名を hardcode (`Misskey.queueTypes`) で
// 列挙しており、mk-go の asynq queue 名 (deliver/push/maintenance/webhook/
// export) と完全には一致しない。存在しない queue を叩かれたときは 500 で
// はなくゼロ埋めの shape を返して、フロント側の queueInfo が stale に
// ならないようにする。
func (h *Handler) QueueQueueStats(c echo.Context) error {
	if h.queueInspector == nil {
		return c.JSON(http.StatusOK, map[string]any{})
	}
	var req struct {
		Queue string `json:"queue"`
	}
	_ = c.Bind(&req)
	if req.Queue != "" {
		info, err := h.queueInspector.GetQueueInfo(req.Queue)
		if err != nil || info == nil {
			return c.JSON(http.StatusOK, shapeQueueForFrontend(&QueueInfoResult{Queue: req.Queue}))
		}
		return c.JSON(http.StatusOK, shapeQueueForFrontend(info))
	}
	queues, err := h.queueInspector.Queues()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	out := map[string]any{}
	for _, q := range queues {
		info, err := h.queueInspector.GetQueueInfo(q)
		if err != nil {
			continue
		}
		out[q] = shapeQueueForFrontend(info)
	}
	return c.JSON(http.StatusOK, out)
}

// QueueQueues handles POST /api/admin/queue/queues.
func (h *Handler) QueueQueues(c echo.Context) error {
	if h.queueInspector == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	queues, err := h.queueInspector.Queues()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	result := make([]map[string]any, 0, len(queues))
	for _, q := range queues {
		info, err := h.queueInspector.GetQueueInfo(q)
		if err != nil {
			continue
		}
		result = append(result, shapeQueueForFrontend(info))
	}
	return c.JSON(http.StatusOK, result)
}

// QueueRemoveJob handles POST /api/admin/queue/remove-job.
func (h *Handler) QueueRemoveJob(c echo.Context) error {
	var req struct {
		Queue string `json:"queue"`
		ID    string `json:"id"`
	}
	if err := c.Bind(&req); err != nil || req.Queue == "" || req.ID == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "queue and id are required.", "00000000-0000-0000-0000-000000000000"))
	}
	if h.queueInspector == nil {
		return c.NoContent(http.StatusNoContent)
	}
	if err := h.queueInspector.DeleteTask(req.Queue, req.ID); err != nil {
		return c.JSON(http.StatusNotFound, apierr.Error("NOT_FOUND", "Not found.", "00000000-0000-0000-0000-000000000000"))
	}
	return c.NoContent(http.StatusNoContent)
}

// QueueRetryJob handles POST /api/admin/queue/retry-job.
func (h *Handler) QueueRetryJob(c echo.Context) error {
	var req struct {
		Queue string `json:"queue"`
		ID    string `json:"id"`
	}
	if err := c.Bind(&req); err != nil || req.Queue == "" || req.ID == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "queue and id are required.", "00000000-0000-0000-0000-000000000000"))
	}
	if h.queueInspector == nil {
		return c.NoContent(http.StatusNoContent)
	}
	if err := h.queueInspector.RunTask(req.Queue, req.ID); err != nil {
		return c.JSON(http.StatusNotFound, apierr.Error("NOT_FOUND", "Not found.", "00000000-0000-0000-0000-000000000000"))
	}
	return c.NoContent(http.StatusNoContent)
}

// QueueShowJob handles POST /api/admin/queue/show-job.
func (h *Handler) QueueShowJob(c echo.Context) error {
	var req struct {
		Queue string `json:"queue"`
		ID    string `json:"id"`
	}
	if err := c.Bind(&req); err != nil || req.Queue == "" || req.ID == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "queue and id are required.", "00000000-0000-0000-0000-000000000000"))
	}
	if h.queueInspector == nil {
		return c.JSON(http.StatusNotFound, apierr.Error("NOT_FOUND", "Not found.", "00000000-0000-0000-0000-000000000000"))
	}
	t, err := h.queueInspector.GetTaskInfo(req.Queue, req.ID)
	if err != nil {
		return c.JSON(http.StatusNotFound, apierr.Error("NOT_FOUND", "Not found.", "00000000-0000-0000-0000-000000000000"))
	}
	return c.JSON(http.StatusOK, packTaskSummary(t))
}

// QueueShowJobLogs handles POST /api/admin/queue/show-job-logs.
// asynq does not persist per-task log output. Returns an empty array to keep
// the admin UI usable without extra infra.
func (h *Handler) QueueShowJobLogs(c echo.Context) error { return c.JSON(http.StatusOK, []any{}) }

// packTaskSummary normalizes a QueueTaskSummary into the Misskey Bull-shaped
// JSON expected by admin/job-queue.vue and job-queue.job.vue. frontend は
// `job.stacktrace.length` / `job.opts.attempts` / `job.opts.repeat` などを
// 直接参照するので、未設定フィールドは undefined ではなく空配列 / 0 / nil
// で埋めて render crash を防ぐ。
//
// asynqにしか無い field (state / queue / payload raw 等) は残しつつ、
// Bull 互換 field を追加する形で出力する (両方を見るadmin toolへの配慮)。
func packTaskSummary(t *QueueTaskSummary) map[string]any {
	if t == nil {
		return nil
	}
	isFailed := t.LastErr != ""
	// asynqはBullと違いEnqueuedAtを保持しない (TaskInfoに該当field無し)。
	// frontend の MkTl / MkTime は 0 を「たった今」として扱うので害はない。
	pack := map[string]any{
		// Bull 互換 field (frontend 必須)
		"id":           t.ID,
		"name":         t.Type,
		"timestamp":    formatUnixMillisOrZero(t.NextProcessAt),
		"processedAt":  formatUnixMillisOrZero(t.LastFailedAt),
		"processedOn":  formatUnixMillisOrZero(t.LastFailedAt),
		"finishedOn":   formatUnixMillisOrZero(t.CompletedAt),
		"progress":     0,
		"attempts":     t.Retried,
		"attemptsMade": t.Retried,
		"isFailed":     isFailed,
		"delay":        0,
		"returnValue":  nil,
		"stacktrace":   stacktraceFrom(t.LastErr),
		"data":         rawJSONOrString(t.Payload),
		"opts": map[string]any{
			"attempts": t.MaxRetry,
			"delay":    0,
			"repeat":   nil,
		},
		// asynq-native field (既存 admin tool 互換のために残す)
		"queue":    t.Queue,
		"type":     t.Type,
		"state":    t.State,
		"payload":  string(t.Payload),
		"retried":  t.Retried,
		"maxRetry": t.MaxRetry,
	}
	if t.LastErr != "" {
		pack["lastErr"] = t.LastErr
		pack["failedReason"] = t.LastErr
	}
	if !t.LastFailedAt.IsZero() {
		pack["lastFailedAt"] = t.LastFailedAt.UTC().Format(time.RFC3339Nano)
	}
	if !t.NextProcessAt.IsZero() {
		pack["nextProcessAt"] = t.NextProcessAt.UTC().Format(time.RFC3339Nano)
	}
	if !t.CompletedAt.IsZero() {
		pack["completedAt"] = t.CompletedAt.UTC().Format(time.RFC3339Nano)
	}
	return pack
}

// formatUnixMillisOrZero returns the unix-ms representation of t, or 0 when
// t is the zero time. Bull's timestamps are unix milliseconds.
func formatUnixMillisOrZero(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

// stacktraceFrom returns a single-element stacktrace array when lastErr is
// non-empty, otherwise an empty array. frontend は `job.stacktrace.length`
// を見るため undefined では TypeError で crash する。
func stacktraceFrom(lastErr string) []string {
	if lastErr == "" {
		return []string{}
	}
	return []string{lastErr}
}

// rawJSONOrString attempts to decode payload as JSON so the admin UI can
// render structured data. Falls back to a string representation for payloads
// that are not valid JSON.
func rawJSONOrString(payload []byte) any {
	if len(payload) == 0 {
		return nil
	}
	var decoded any
	if err := json.Unmarshal(payload, &decoded); err == nil {
		return decoded
	}
	return string(payload)
}

// QueueStats handles POST /api/admin/queue/stats.
func (h *Handler) QueueStats(c echo.Context) error {
	if h.queueInspector == nil {
		return c.JSON(http.StatusOK, map[string]any{
			"deliver": map[string]any{"activeSince": nil, "active": 0, "waiting": 0, "delayed": 0},
			"inbox":   map[string]any{"activeSince": nil, "active": 0, "waiting": 0, "delayed": 0},
		})
	}
	result := map[string]any{}
	for _, qname := range []string{"deliver", "inbox"} {
		info, err := h.queueInspector.GetQueueInfo(qname)
		if err != nil {
			result[qname] = map[string]any{"activeSince": nil, "active": 0, "waiting": 0, "delayed": 0}
			continue
		}
		result[qname] = map[string]any{
			"activeSince": nil, "active": info.Active, "waiting": info.Pending, "delayed": info.Scheduled,
		}
	}
	return c.JSON(http.StatusOK, result)
}

// --- relays ---

// RelaysAdd handles POST /api/admin/relays/add. Routes through the
// relay Service so that a Follow activity is actually dispatched to
// the relay's inbox (#161). Falls back to raw DB insertion when the
// service is not wired (tests or early boot).
func (h *Handler) RelaysAdd(c echo.Context) error {
	var req struct {
		Inbox string `json:"inbox"`
	}
	_ = c.Bind(&req)
	if req.Inbox == "" {
		return c.NoContent(http.StatusNoContent)
	}
	if h.relayService != nil {
		rel, err := h.relayService.Add(c.Request().Context(), req.Inbox)
		if err != nil {
			return apierr.JSONInternalError(c)
		}
		return c.JSON(http.StatusOK, rel)
	}
	if h.adminDB == nil {
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
	if h.relayService != nil {
		list, err := h.relayService.List(c.Request().Context())
		if err != nil {
			return apierr.JSONInternalError(c)
		}
		if list == nil {
			list = []*model.Relay{}
		}
		return c.JSON(http.StatusOK, list)
	}
	if h.adminDB == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	var relays []*model.Relay
	h.adminDB.Order(`"id" DESC`).Find(&relays)
	return c.JSON(http.StatusOK, relays)
}

// RelaysRemove handles POST /api/admin/relays/remove. When the relay
// service is configured, it sends an Undo(Follow) to the relay inbox
// before deleting the row.
func (h *Handler) RelaysRemove(c echo.Context) error {
	var req struct {
		ID    string `json:"id"`
		Inbox string `json:"inbox"`
	}
	_ = c.Bind(&req)
	if h.relayService != nil {
		id := req.ID
		if id == "" && req.Inbox != "" && h.adminDB != nil {
			// 本家互換のため inbox 指定でも remove できるようにする
			var rel model.Relay
			if err := h.adminDB.Where(`"inbox" = ?`, req.Inbox).First(&rel).Error; err == nil {
				id = rel.ID
			}
		}
		if id == "" {
			return c.NoContent(http.StatusNoContent)
		}
		if err := h.relayService.Remove(c.Request().Context(), id); err != nil {
			return apierr.JSONInternalError(c)
		}
		return c.NoContent(http.StatusNoContent)
	}
	if h.adminDB == nil {
		return c.NoContent(http.StatusNoContent)
	}
	h.adminDB.Where(`"id" = ?`, req.ID).Delete(&model.Relay{})
	return c.NoContent(http.StatusNoContent)
}

// --- system-webhook ---

// SystemWebhookCreate handles POST /api/admin/system-webhook/create.
func (h *Handler) SystemWebhookCreate(c echo.Context) error {
	if h.systemWebhookRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		Name     string   `json:"name"`
		URL      string   `json:"url"`
		Secret   string   `json:"secret"`
		On       []string `json:"on"`
		IsActive *bool    `json:"isActive"`
	}
	if err := c.Bind(&req); err != nil || req.Name == "" || req.URL == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "Invalid parameters.", "00000000-0000-0000-0000-000000000000"))
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	sw := &model.SystemWebhook{
		ID:        h.idGen.Generate(time.Now()),
		Name:      req.Name,
		URL:       req.URL,
		Secret:    req.Secret,
		On:        req.On,
		IsActive:  isActive,
		UpdatedAt: time.Now(),
	}
	if err := h.systemWebhookRepo.Create(sw); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	return c.JSON(http.StatusOK, sw)
}

// SystemWebhookDelete handles POST /api/admin/system-webhook/delete.
func (h *Handler) SystemWebhookDelete(c echo.Context) error {
	if h.systemWebhookRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		ID string `json:"id"`
	}
	_ = c.Bind(&req)
	_ = h.systemWebhookRepo.Delete(req.ID)
	return c.NoContent(http.StatusNoContent)
}

// SystemWebhookList handles POST /api/admin/system-webhook/list.
func (h *Handler) SystemWebhookList(c echo.Context) error {
	if h.systemWebhookRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	rows, err := h.systemWebhookRepo.List()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	if rows == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	return c.JSON(http.StatusOK, rows)
}

// SystemWebhookShow handles POST /api/admin/system-webhook/show.
func (h *Handler) SystemWebhookShow(c echo.Context) error {
	if h.systemWebhookRepo == nil {
		return c.JSON(http.StatusNotFound, apierr.Error("NOT_FOUND", "Not found.", "00000000-0000-0000-0000-000000000000"))
	}
	var req struct {
		ID string `json:"id"`
	}
	_ = c.Bind(&req)
	sw, err := h.systemWebhookRepo.FindByID(req.ID)
	if err != nil {
		return c.JSON(http.StatusNotFound, apierr.Error("NOT_FOUND", "Not found.", "00000000-0000-0000-0000-000000000000"))
	}
	return c.JSON(http.StatusOK, sw)
}

// SystemWebhookTest handles POST /api/admin/system-webhook/test.
func (h *Handler) SystemWebhookTest(c echo.Context) error {
	var req struct {
		WebhookID string `json:"webhookId"`
		Type      string `json:"type"`
	}
	_ = c.Bind(&req)
	if req.WebhookID == "" || h.systemWebhookRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	sw, err := h.systemWebhookRepo.FindByID(req.WebhookID)
	if err != nil {
		return c.NoContent(http.StatusNoContent)
	}
	// テスト送信(非同期)。配送結果は latestStatus 系カラムに反映されないが、
	// Misskey 本家の /system-webhook/test も fire-and-forget 挙動なので整合。
	go sendWebhookTest(sw.URL, sw.Secret, req.Type)
	return c.NoContent(http.StatusNoContent)
}

// SystemWebhookUpdate handles POST /api/admin/system-webhook/update.
//
// 配送 processor が並行して latestSentAt/latestStatus を書き換えるため、
// FindByID→Save で全列上書きすると配送ステータスを古い値で踏み潰す。partial
// update (UpdateAdminFields) を使い admin 編集可能列のみ触る。
func (h *Handler) SystemWebhookUpdate(c echo.Context) error {
	if h.systemWebhookRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	var req struct {
		ID       string    `json:"id"`
		Name     *string   `json:"name"`
		URL      *string   `json:"url"`
		Secret   *string   `json:"secret"`
		On       *[]string `json:"on"`
		IsActive *bool     `json:"isActive"`
	}
	if err := c.Bind(&req); err != nil || req.ID == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "Invalid parameters.", "00000000-0000-0000-0000-000000000000"))
	}
	// 存在確認 (GORM Updates(map) は 0 行影響でも nil を返すため)
	if _, err := h.systemWebhookRepo.FindByID(req.ID); err != nil {
		return c.JSON(http.StatusNotFound, apierr.Error("NOT_FOUND", "Not found.", "00000000-0000-0000-0000-000000000000"))
	}
	fields := map[string]any{"updatedAt": time.Now()}
	if req.Name != nil {
		fields["name"] = *req.Name
	}
	if req.URL != nil {
		fields["url"] = *req.URL
	}
	if req.Secret != nil {
		fields["secret"] = *req.Secret
	}
	if req.On != nil {
		fields["on"] = *req.On
	}
	if req.IsActive != nil {
		fields["isActive"] = *req.IsActive
	}
	if err := h.systemWebhookRepo.UpdateAdminFields(req.ID, fields); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	sw, err := h.systemWebhookRepo.FindByID(req.ID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	return c.JSON(http.StatusOK, sw)
}
