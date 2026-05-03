package admin

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	"github.com/shiroha-a/mk/internal/core/moderationlog"
)

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
	// snapshot for moderation log info (forwarded フラグが立つ前の状態)
	var snapshot any
	if h.abuseRepo != nil {
		if r, _ := h.abuseRepo.FindByID(req.ReportID); r != nil {
			snapshot = r
		}
	}
	if h.abuseForwarder != nil {
		if err := h.abuseForwarder.ForwardReport(req.ReportID); err != nil {
			return apierr.JSONInternalError(c)
		}
		if snapshot != nil {
			h.logModeration(c, moderationlog.LogForwardAbuseReport, map[string]any{
				"reportId": req.ReportID,
				"report":   snapshot,
			})
		}
		return c.NoContent(http.StatusNoContent)
	}
	// forwarder 未配線時のフォールバック: DB フラグだけ更新する (テストや
	// federation stack 未初期化パスで有効)。
	if h.abuseRepo != nil {
		if err := h.abuseRepo.UpdateFields(req.ReportID, map[string]any{"forwarded": true}); err == nil && snapshot != nil {
			h.logModeration(c, moderationlog.LogForwardAbuseReport, map[string]any{
				"reportId": req.ReportID,
				"report":   snapshot,
			})
		}
	}
	return c.NoContent(http.StatusNoContent)
}
