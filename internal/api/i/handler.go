package i

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// Handler handles account-related API endpoints.
type Handler struct {
	userRepo repository.UserRepository
}

// NewHandler creates a new account Handler.
func NewHandler(userRepo repository.UserRepository) *Handler {
	return &Handler{userRepo: userRepo}
}

// Me handles POST /api/i - returns the authenticated user's info.
func (h *Handler) Me(c echo.Context) error {
	user := middleware.GetUser(c)

	profile, _ := h.userRepo.FindProfileByUserID(user.ID)

	detailed := entity.PackUserDetailed(user, profile)

	// /api/i returns additional private fields
	resp := map[string]any{
		"id":                user.ID,
		"name":              detailed.Name,
		"username":          detailed.Username,
		"host":              detailed.Host,
		"avatarUrl":         detailed.AvatarURL,
		"avatarBlurhash":    detailed.AvatarBlurhash,
		"avatarDecorations": detailed.AvatarDecorations,
		"isBot":             detailed.IsBot,
		"isCat":             detailed.IsCat,
		"emojis":            detailed.Emojis,
		"onlineStatus":      detailed.OnlineStatus,
		"badgeRoles":        detailed.BadgeRoles,
		"bannerUrl":         detailed.BannerURL,
		"bannerBlurhash":    detailed.BannerBlurhash,
		"isLocked":          detailed.IsLocked,
		"isSuspended":       detailed.IsSuspended,
		"description":       detailed.Description,
		"location":          detailed.Location,
		"birthday":          detailed.Birthday,
		"lang":              detailed.Lang,
		"fields":            detailed.Fields,
		"followersCount":    detailed.FollowersCount,
		"followingCount":    detailed.FollowingCount,
		"notesCount":        detailed.NotesCount,
	}

	// Private fields
	if profile != nil {
		resp["email"] = profile.Email
		resp["emailVerified"] = profile.EmailVerified
		resp["autoAcceptFollowed"] = profile.AutoAcceptFollowed
		resp["noCrawle"] = profile.NoCrawle
		resp["preventAiLearning"] = profile.PreventAiLearning
		resp["alwaysMarkNsfw"] = profile.AlwaysMarkNsfw
		resp["autoSensitive"] = profile.AutoSensitive
		resp["carefulBot"] = profile.CarefulBot
		resp["injectFeaturedNote"] = profile.InjectFeaturedNote
		resp["receiveAnnouncementEmail"] = profile.ReceiveAnnouncementEmail
		resp["twoFactorEnabled"] = profile.TwoFactorEnabled
		resp["securityKeysAvailable"] = profile.SecurityKeysAvailable
		resp["usePasswordLessLogin"] = profile.UsePasswordLessLogin
	}
	resp["hasUnreadNotification"] = false
	resp["hasPendingReceivedFollowRequest"] = false

	return c.JSON(http.StatusOK, resp)
}
