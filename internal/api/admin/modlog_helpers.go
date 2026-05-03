package admin

import (
	"log/slog"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/core/moderationlog"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// logModeration is the generic moderation log writer. Callers build the
// info map themselves to match the type-specific schema expected by the
// frontend (suspend → {userId, userUsername, userHost}, updateRole →
// {roleId, before, after}, etc.).
//
// fire-and-forget: no error returned. All failure modes — unwired
// service, missing actor (RequireModerator misconfiguration) — are
// silently dropped or logged at warn level so audit logging never
// blocks or fails the moderation action itself.
//
// Callers should invoke this only AFTER the underlying moderation
// action has succeeded so the log reflects observed state changes
// rather than attempts.
func (h *Handler) logModeration(c echo.Context, t moderationlog.LogType, info map[string]any) {
	if h.modLogService == nil {
		return
	}
	ctx := c.Request().Context()
	actor := middleware.GetUser(c)
	if actor == nil {
		// RequireModerator middleware should guarantee an actor; if we
		// hit this branch in production something is misconfigured and
		// we want it visible rather than silently dropped.
		slog.WarnContext(ctx, "moderation log: skipping — actor missing in moderator-only handler",
			"type", t)
		return
	}
	h.modLogService.Log(ctx, actor.ID, t, info)
}

// logUserAction records a moderation_log entry for an action that
// targets a single user. The {userId, userUsername, userHost} payload
// is built from a fresh user lookup, matching the Misskey TS frontend
// schema for user-targeted log types (suspend, unsuspend, resetPassword,
// deleteAccount, unsetUserAvatar, unsetUserBanner).
//
// Convenience wrapper around logModeration. For user-targeted log
// types that need extra fields (updateUserNote with before/after,
// assignRole with role info, etc.), build the info map yourself and
// call logModeration directly.
func (h *Handler) logUserAction(c echo.Context, t moderationlog.LogType, targetUserID string) {
	if h.userRepo == nil {
		return
	}
	target, err := h.userRepo.FindByID(targetUserID)
	if err != nil || target == nil {
		return
	}
	h.logModeration(c, t, moderationlog.UserInfo(target))
}

// logRoleAssignment records a moderation_log entry for assignRole /
// unassignRole. Combines UserInfo with role identifier/name; the
// frontend's modlog.ModLog.vue branch reads {userId, userUsername,
// userHost, roleId, roleName} for these types. roleService lookup
// failures are tolerated (best-effort): the user info is logged even
// if the role snapshot is unavailable.
func (h *Handler) logRoleAssignment(c echo.Context, t moderationlog.LogType, targetUserID, roleID string) {
	if h.userRepo == nil {
		return
	}
	target, err := h.userRepo.FindByID(targetUserID)
	if err != nil || target == nil {
		return
	}
	info := moderationlog.UserInfo(target)
	info["roleId"] = roleID
	if h.roleService != nil {
		if r, err := h.roleService.Show(roleID); err == nil && r != nil {
			info["roleName"] = r.Name
		}
	}
	h.logModeration(c, t, info)
}
