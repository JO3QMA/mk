package admin

import (
	"log/slog"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/core/moderationlog"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// logUserAction records a moderation_log entry for an action that
// targets a single user. The {userId, userUsername, userHost} payload
// is built from a fresh user lookup, matching the Misskey TS frontend
// schema for user-targeted log types (suspend, unsuspend, resetPassword,
// updateUserNote, deleteAccount, unsetUserAvatar, unsetUserBanner).
//
// fire-and-forget: no error returned. All failure modes — unwired
// service, missing actor (RequireModerator misconfiguration), missing
// target user — are silently dropped or logged at warn level so audit
// logging never blocks or fails the moderation action itself.
//
// Callers should invoke this only AFTER the underlying moderation
// action has succeeded (e.g. after the suspend flag actually flipped),
// so the log reflects observed state changes rather than attempts.
func (h *Handler) logUserAction(c echo.Context, t moderationlog.LogType, targetUserID string) {
	if h.modLogService == nil || h.userRepo == nil {
		return
	}
	ctx := c.Request().Context()
	actor := middleware.GetUser(c)
	if actor == nil {
		// RequireModerator middleware should guarantee an actor; if we
		// hit this branch in production something is misconfigured and
		// we want it visible rather than silently dropped.
		slog.WarnContext(ctx, "moderation log: skipping — actor missing in moderator-only handler",
			"type", t, "targetUserId", targetUserID)
		return
	}
	target, err := h.userRepo.FindByID(targetUserID)
	if err != nil || target == nil {
		return
	}
	h.modLogService.Log(ctx, actor.ID, t, moderationlog.UserInfo(target))
}
