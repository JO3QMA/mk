package federation

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
)

// UpdateRemoteUser handles POST /api/federation/update-remote-user.
//
// 対象のリモートユーザーを ActivityPub actor fetch で強制更新する。管理者用。
func (h *Handler) UpdateRemoteUser(c echo.Context) error {
	var req struct {
		UserID string `json:"userId"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return apierr.JSONInvalidParam(c)
	}
	if h.userRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	user, err := h.userRepo.FindByID(req.UserID)
	if err != nil {
		return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_USER", "No such user.", "ae1bd95a-1a3b-4d4e-9c47-7e76a2020f8e"))
	}
	if user.URI == nil || *user.URI == "" {
		// ローカルユーザーは対象外。
		return c.NoContent(http.StatusNoContent)
	}
	if h.resolver == nil {
		return c.NoContent(http.StatusNoContent)
	}
	if _, err := h.resolver.ForceResolveActor(*user.URI); err != nil {
		return apierr.JSONInternalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}
