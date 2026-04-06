package i

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/misskey-dev/misskey-go/internal/entity"
	"github.com/misskey-dev/misskey-go/internal/model"
	"github.com/misskey-dev/misskey-go/internal/server/middleware"
	"gorm.io/gorm"
)

// Handler handles account-related API endpoints.
type Handler struct {
	db *gorm.DB
}

// NewHandler creates a new account Handler.
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{db: db}
}

// Me handles POST /api/i - returns the authenticated user's info.
func (h *Handler) Me(c echo.Context) error {
	user := middleware.GetUser(c)

	var profile model.UserProfile
	h.db.First(&profile, "\"userId\" = ?", user.ID)

	detailed := entity.PackUserDetailed(user, &profile)

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
		// Private fields
		"email":                           profile.Email,
		"emailVerified":                   profile.EmailVerified,
		"autoAcceptFollowed":              profile.AutoAcceptFollowed,
		"noCrawle":                        profile.NoCrawle,
		"preventAiLearning":               profile.PreventAiLearning,
		"alwaysMarkNsfw":                  profile.AlwaysMarkNsfw,
		"autoSensitive":                   profile.AutoSensitive,
		"carefulBot":                      profile.CarefulBot,
		"injectFeaturedNote":              profile.InjectFeaturedNote,
		"receiveAnnouncementEmail":        profile.ReceiveAnnouncementEmail,
		"twoFactorEnabled":                profile.TwoFactorEnabled,
		"securityKeysAvailable":           profile.SecurityKeysAvailable,
		"usePasswordLessLogin":            profile.UsePasswordLessLogin,
		"hasUnreadNotification":           false,
		"hasPendingReceivedFollowRequest": false,
	}

	return c.JSON(http.StatusOK, resp)
}
