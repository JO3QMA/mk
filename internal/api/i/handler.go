package i

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/core/user"
	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// Handler handles account-related API endpoints.
type Handler struct {
	userService *user.Service
	idGen       id.Generator
}

// NewHandler creates a new account Handler.
func NewHandler(userService *user.Service, idGen id.Generator) *Handler {
	return &Handler{userService: userService, idGen: idGen}
}

// Me handles POST /api/i - returns the authenticated user's info.
func (h *Handler) Me(c echo.Context) error {
	u := middleware.GetUser(c)

	profile := h.userService.GetProfile(u.ID)

	detailed := entity.PackUserDetailed(u, profile)

	// avatarUrl が未設定の場合は identicon URL を生成
	avatarURL := detailed.AvatarURL
	if avatarURL == nil {
		ident := fmt.Sprintf("/identicon/%s", u.Username)
		avatarURL = &ident
	}

	// /api/i returns additional private fields
	resp := map[string]any{
		// UserLite fields
		"id":                u.ID,
		"name":              detailed.Name,
		"username":          detailed.Username,
		"host":              detailed.Host,
		"avatarUrl":         avatarURL,
		"avatarBlurhash":    detailed.AvatarBlurhash,
		"avatarDecorations": detailed.AvatarDecorations,
		"isBot":             detailed.IsBot,
		"isCat":             detailed.IsCat,
		"emojis":            detailed.Emojis,
		"onlineStatus":      detailed.OnlineStatus,
		"badgeRoles":        detailed.BadgeRoles,
		// UserDetailed fields
		"bannerUrl":      detailed.BannerURL,
		"bannerBlurhash": detailed.BannerBlurhash,
		"isLocked":       detailed.IsLocked,
		"isSilenced":     u.IsSuspended && false, // ロールベースで判定 (未実装)
		"isSuspended":    detailed.IsSuspended,
		"description":    detailed.Description,
		"location":       detailed.Location,
		"birthday":       detailed.Birthday,
		"lang":           detailed.Lang,
		"fields":         detailed.Fields,
		"verifiedLinks":  []string{},
		"followersCount": detailed.FollowersCount,
		"followingCount": detailed.FollowingCount,
		"notesCount":     detailed.NotesCount,
		"uri":            detailed.URI,
		"url":            detailed.URL,
		"movedTo":        nil,
		"alsoKnownAs":    nil,
		"updatedAt":      detailed.UpdatedAt,
		"lastFetchedAt":  nil,
		// MeDetailed fields
		"avatarId":            nil,
		"bannerId":            nil,
		"followersVisibility": "public",
		"followingVisibility": "public",
		"chatScope":           "mutual",
		"canChat":             true,
		"followedMessage":     nil,
		"memo":                nil,
		"moderationNote":      nil,
		"hideOnlineStatus":    u.HideOnlineStatus,
	}

	// Private fields from profile
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
		resp["usePasswordLessLogin"] = profile.UsePasswordLessLogin
		resp["mutedWords"] = profile.MutedWords
		resp["hardMutedWords"] = profile.HardMutedWords
		resp["mutedInstances"] = profile.MutedInstances
		resp["publicReactions"] = profile.PublicReactions
	}

	// フロントエンド互換性フィールド (Phase 4.5c)
	resp["isAdmin"] = false
	resp["isModerator"] = false
	resp["isDeleted"] = u.IsDeleted
	resp["isExplorable"] = u.IsExplorable
	resp["hasUnreadNotification"] = false
	resp["hasPendingReceivedFollowRequest"] = false
	resp["hasUnreadAnnouncement"] = false
	resp["hasUnreadAntenna"] = false
	resp["hasUnreadChannel"] = false
	resp["hasUnreadMentions"] = false
	resp["hasUnreadSpecifiedNotes"] = false
	resp["hasUnreadChatMessages"] = false
	resp["unreadNotificationsCount"] = 0
	resp["unreadAnnouncements"] = []any{}
	resp["pinnedNoteIds"] = []string{}
	resp["pinnedNotes"] = []any{}
	resp["pinnedPageId"] = nil
	resp["pinnedPage"] = nil
	resp["loggedInDays"] = 0
	resp["policies"] = defaultMePolicies()
	resp["roles"] = []any{}
	resp["achievements"] = []any{}
	resp["twoFactorBackupCodesStock"] = "none"
	resp["securityKeys"] = false
	resp["securityKeysList"] = []any{}
	resp["mutingNotificationTypes"] = []any{}
	resp["notificationRecieveConfig"] = map[string]any{}
	resp["emailNotificationTypes"] = []string{"follow", "receiveFollowRequest"}

	// createdAt は ID から復元
	if t, err := h.idGen.ParseTime(u.ID); err == nil {
		resp["createdAt"] = t.UTC().Format("2006-01-02T15:04:05.000Z")
	}

	return c.JSON(http.StatusOK, resp)
}

// UpdateRequest is the request body for i/update.
// 各フィールドはポインタで「未指定なら変更しない」セマンティクスを持つ。
// 文字列のnullable化はrawMessageでなくJSONで対応するため省略している。
type UpdateRequest struct {
	Name              *string `json:"name"`
	Description       *string `json:"description"`
	Location          *string `json:"location"`
	Birthday          *string `json:"birthday"`
	Lang              *string `json:"lang"`
	IsLocked          *bool   `json:"isLocked"`
	IsBot             *bool   `json:"isBot"`
	IsCat             *bool   `json:"isCat"`
	IsExplorable      *bool   `json:"isExplorable"`
	HideOnlineStatus  *bool   `json:"hideOnlineStatus"`
	AlwaysMarkNsfw    *bool   `json:"alwaysMarkNsfw"`
	AutoSensitive     *bool   `json:"autoSensitive"`
	NoCrawle          *bool   `json:"noCrawle"`
	PreventAiLearning *bool   `json:"preventAiLearning"`
}

// Update handles POST /api/i/update.
func (h *Handler) Update(c echo.Context) error {
	me := middleware.GetUser(c)

	var req UpdateRequest
	if err := c.Bind(&req); err != nil {
		return invalidParam(c)
	}

	in := user.UpdateInput{
		IsLocked:          req.IsLocked,
		IsBot:             req.IsBot,
		IsCat:             req.IsCat,
		IsExplorable:      req.IsExplorable,
		HideOnlineStatus:  req.HideOnlineStatus,
		AlwaysMarkNsfw:    req.AlwaysMarkNsfw,
		AutoSensitive:     req.AutoSensitive,
		NoCrawle:          req.NoCrawle,
		PreventAiLearning: req.PreventAiLearning,
	}
	if req.Name != nil {
		in.Name = &req.Name
	}
	if req.Description != nil {
		in.Description = &req.Description
	}
	if req.Location != nil {
		in.Location = &req.Location
	}
	if req.Birthday != nil {
		in.Birthday = &req.Birthday
	}
	if req.Lang != nil {
		in.Lang = &req.Lang
	}

	bundle, err := h.userService.UpdateProfile(me.ID, in)
	if err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			return c.JSON(http.StatusNotFound, errEnvelope("No such user.", "NO_SUCH_USER", "4362f8dc-731f-4ad8-a694-be5a88922a24"))
		}
		return internalError(c)
	}

	return c.JSON(http.StatusOK, entity.PackUserDetailed(bundle.User, bundle.Profile))
}

// PinRequest is the request body for i/pin and i/unpin.
type PinRequest struct {
	NoteID string `json:"noteId"`
}

// Pin handles POST /api/i/pin.
func (h *Handler) Pin(c echo.Context) error {
	me := middleware.GetUser(c)

	var req PinRequest
	if err := c.Bind(&req); err != nil || req.NoteID == "" {
		return invalidParam(c)
	}

	if err := h.userService.PinNote(me.ID, req.NoteID); err != nil {
		switch {
		case errors.Is(err, user.ErrNoteNotFound):
			return c.JSON(http.StatusNotFound, errEnvelope("No such note.", "NO_SUCH_NOTE", "56734f8b-3928-431e-bf80-6ff87df40cb3"))
		case errors.Is(err, user.ErrAlreadyPinned):
			return c.JSON(http.StatusBadRequest, errEnvelope("That note has already been pinned.", "ALREADY_PINNED", "8b18c2b7-68fe-4edb-9892-c0cbaeb6c913"))
		case errors.Is(err, user.ErrPinLimitExceeded):
			return c.JSON(http.StatusBadRequest, errEnvelope("You can not pin notes any more.", "PIN_LIMIT_EXCEEDED", "72dab508-c64d-498f-8740-a8eec1ba385a"))
		default:
			return internalError(c)
		}
	}

	return h.Me(c)
}

// Unpin handles POST /api/i/unpin.
func (h *Handler) Unpin(c echo.Context) error {
	me := middleware.GetUser(c)

	var req PinRequest
	if err := c.Bind(&req); err != nil || req.NoteID == "" {
		return invalidParam(c)
	}

	if err := h.userService.UnpinNote(me.ID, req.NoteID); err != nil {
		if errors.Is(err, user.ErrPinNotFound) {
			return c.JSON(http.StatusNotFound, errEnvelope("No such note.", "NO_SUCH_NOTE", "454170ce-44d9-4d2a-94fc-e6854ec2f7d1"))
		}
		return internalError(c)
	}

	return h.Me(c)
}

func invalidParam(c echo.Context) error {
	return c.JSON(http.StatusBadRequest, errEnvelope("Invalid param.", "INVALID_PARAM", "3d81ceae-475f-4600-b2a8-2bc116157532"))
}

func internalError(c echo.Context) error {
	return c.JSON(http.StatusInternalServerError, errEnvelope("Internal error.", "INTERNAL_ERROR", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
}

func errEnvelope(message, code, id string) map[string]any {
	return map[string]any{
		"error": map[string]any{
			"message": message,
			"code":    code,
			"id":      id,
		},
	}
}

// defaultMePolicies returns the Misskey default policies for MeDetailed.
func defaultMePolicies() map[string]any {
	return map[string]any{
		"gtlAvailable":               true,
		"ltlAvailable":               true,
		"canPublicNote":              true,
		"mentionLimit":               20,
		"canInvite":                  false,
		"inviteLimit":                0,
		"inviteLimitCycle":           10080,
		"inviteExpirationTime":       0,
		"canManageCustomEmojis":      false,
		"canManageAvatarDecorations": false,
		"canSearchNotes":             false,
		"canSearchUsers":             true,
		"canUseTranslator":           true,
		"canHideAds":                 false,
		"driveCapacityMb":            100,
		"maxFileSizeMb":              30,
		"alwaysMarkNsfw":             false,
		"canUpdateBioMedia":          true,
		"pinLimit":                   5,
		"antennaLimit":               5,
		"wordMuteLimit":              200,
		"webhookLimit":               3,
		"clipLimit":                  10,
		"noteEachClipsLimit":         200,
		"userListLimit":              10,
		"userEachUserListsLimit":     50,
		"rateLimitFactor":            1,
		"avatarDecorationLimit":      1,
		"canImportAntennas":          false,
		"canImportBlocking":          false,
		"canImportFollowing":         false,
		"canImportMuting":            false,
		"canImportUserLists":         false,
		"chatAvailability":           "available",
		"uploadableFileTypes":        []string{"text/*", "application/json", "image/*", "video/*", "audio/*"},
		"noteDraftLimit":             10,
		"scheduledNoteLimit":         1,
		"watermarkAvailable":         true,
	}
}
