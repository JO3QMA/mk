package admin

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	"github.com/shiroha-a/mk/internal/core/moderationlog"
	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/queue"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

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
// Misskey TS 本家 (admin/emoji/copy.ts) は元の name のまま local emoji を
// 追加し、同名の local emoji が既に存在する場合のみ DUPLICATE_NAME を
// 返す。以前の mk-go 実装は `_copy` suffix を勝手に付与していたが、
// これは TS 仕様違反 (#650 問題 1) なのでサフィックスを除去した。
//
// ref: third_party/misskey/packages/backend/src/server/api/endpoints/admin/emoji/copy.ts
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
	if existing, err := h.emojiRepo.FindByNameAndHost(src.Name, nil); err == nil && existing != nil {
		return c.JSON(http.StatusBadRequest, apierr.Error("DUPLICATE_NAME", "Duplicate name.", "f7a3462c-4e6e-4069-8421-b9bd4f4c3975"))
	}
	copied := *src
	copied.ID = h.idGen.Generate(time.Now())
	copied.Host = nil
	if err := h.emojiRepo.Create(&copied); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	h.logModeration(c, moderationlog.LogAddCustomEmoji, map[string]any{
		"emojiId": copied.ID,
		"emoji":   &copied,
	})
	return c.JSON(http.StatusOK, map[string]any{"id": copied.ID})
}

// EmojiDeleteBulk handles POST /api/admin/emoji/delete-bulk.
//
// Misskey TS の CustomEmojiService.deleteBulk() は削除対象を for ループで
// 一件ずつ処理し、絵文字ごとに deleteCustomEmoji moderation log を書く。
// 互換性のため mk-go も削除前の snapshot を取得して per-emoji log を出力
// する。bulk 削除自体は 1 SQL (DeleteMany) で完結。
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
	// per-emoji log のため削除前に snapshot を取得。失敗しても削除自体は
	// 続行する (snapshot 失敗で監査が落ちるより操作完遂が優先)。エラー時は
	// log が出ない理由を debug-level で残して運用時の追跡を可能にする。
	snapshots, err := h.emojiRepo.FindManyByIDs(req.IDs)
	if err != nil {
		slog.DebugContext(c.Request().Context(), "moderation log: snapshot lookup failed before delete-bulk",
			"ids", req.IDs, "err", err)
	}
	if err := h.emojiRepo.DeleteMany(req.IDs); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	for _, e := range snapshots {
		h.logModeration(c, moderationlog.LogDeleteCustomEmoji, map[string]any{
			"emojiId": e.ID,
			"emoji":   e,
		})
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
		Query   string `json:"query"`
		Host    string `json:"host"`
		SinceID string `json:"sinceId"`
		UntilID string `json:"untilId"`
		Limit   int    `json:"limit"`
		Offset  int    `json:"offset"`
	}
	_ = c.Bind(&req)
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 30
	}
	emojis, err := h.emojiRepo.ListRemoteWithFilter(req.Query, req.Host, req.SinceID, req.UntilID, req.Limit, req.Offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "00000000-0000-0000-0000-000000000000"))
	}
	if emojis == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	// upstream Misskey の admin/emoji/list-remote は EmojiDetailed schema を
	// 返す (= `url` field、`publicUrl` ではない)。frontend の旧
	// custom-emojis-manager.vue は emoji.url を読み込んで <img> を組み立てる
	// ため、raw model.Emoji (publicUrl / originalUrl 持ち、url 無し) を返すと
	// 画像が壊れて表示そのものが空に見える (#466)。entity.PackEmojiDetailedList
	// で publicUrl → url 変換した shape にする。
	return c.JSON(http.StatusOK, entity.PackEmojiDetailedList(emojis))
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
