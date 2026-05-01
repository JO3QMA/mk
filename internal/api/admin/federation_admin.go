package admin

import (
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/model"
)

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
// InstanceMetadataFetcher (= coreinstance.FetchMetadataService) 経由で指定ホストの
// nodeinfo + iconUrl / faviconUrl を再取得する。fetcher 未設定または host 未指定
// の場合は no-op で 204 を返す (本家 TS も失敗時エラーコードは返さない挙動)。
func (h *Handler) FederationRefreshRemoteInstanceMetadata(c echo.Context) error {
	var req struct {
		Host string `json:"host"`
	}
	_ = c.Bind(&req)
	if h.instanceMetadataFetcher == nil || req.Host == "" {
		return c.NoContent(http.StatusNoContent)
	}
	// fetch 失敗はユーザーへ明示的にエラー返す必要はない (frontendは成功前提
	// でUI更新するだけ)。ログに残してリトライ可能な状態にしておく。
	if err := h.instanceMetadataFetcher.Fetch(req.Host); err != nil {
		slog.Warn("federation refresh metadata failed", "host", req.Host, "err", err)
	}
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
