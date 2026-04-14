package i

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	"github.com/shiroha-a/mk/internal/core/role"
	"github.com/shiroha-a/mk/internal/core/twofactor"
	"github.com/shiroha-a/mk/internal/core/user"
	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// RoleProvider abstracts role queries for /api/i responses.
// 循環参照を避けるため interface で受け取る。
type RoleProvider interface {
	IsAdministrator(userID string) bool
	IsModerator(userID string) bool
	IsSilenced(userID string) bool
	GetUserRoles(userID string) ([]*model.Role, error)
	GetUserPolicies(userID string) map[string]any
}

// EmailSender sends an email (subject + plain text body). SMTP 設定は
// 実装側が Meta から読み取る。テストではスタブを注入する。
type EmailSender func(to, subject, body string)

// Handler handles account-related API endpoints.
type Handler struct {
	userService      *user.Service
	idGen            id.Generator
	roleProvider     RoleProvider
	registryRepo     repository.RegistryRepository
	favoriteRepo     repository.NoteFavoriteRepository
	transferEnqueuer TransferEnqueuer
	webauthnSvc      *twofactor.WebAuthnService
	securityKeyRepo  repository.UserSecurityKeyRepository
	metaRepo         repository.MetaRepository
	emailSender      EmailSender
	serverURL        string
	signinRepo       repository.SigninRepository
}

// isSilenced returns whether the given user has any role whose merged
// policies deny canPublicNote. Wraps roleProvider so a nil provider
// (early boot / tests that skip role wiring) yields false.
func (h *Handler) isSilenced(userID string) bool {
	if h.roleProvider == nil {
		return false
	}
	return h.roleProvider.IsSilenced(userID)
}

// SetServerURL sets the base URL used for email verification links.
func (h *Handler) SetServerURL(u string) {
	h.serverURL = u
}

// SetEmailSender attaches an EmailSender for update-email verification.
func (h *Handler) SetEmailSender(s EmailSender) {
	h.emailSender = s
}

// SetMetaRepo attaches a MetaRepository. When set, i/update enforces
// meta.prohibitedWordsForNameOfUser against the display name. Tests that
// don't need this validation can leave it unset.
func (h *Handler) SetMetaRepo(r repository.MetaRepository) {
	h.metaRepo = r
}

// SetWebAuthn attaches the WebAuthn service + security key repository.
// Both are required to enable WebAuthn endpoints; if either is nil the
// register/done/remove/update/passwordless handlers return a no-op 204 (so
// existing test fixtures that don't wire the dependency keep passing).
func (h *Handler) SetWebAuthn(svc *twofactor.WebAuthnService, repo repository.UserSecurityKeyRepository) {
	h.webauthnSvc = svc
	h.securityKeyRepo = repo
}

// SetSigninRepo attaches a SigninRepository for i/signin-history.
func (h *Handler) SetSigninRepo(r repository.SigninRepository) {
	h.signinRepo = r
}

// SetFavoriteRepo attaches a NoteFavoriteRepository for i/favorites.
func (h *Handler) SetFavoriteRepo(r repository.NoteFavoriteRepository) {
	h.favoriteRepo = r
}

// NewHandler creates a new account Handler.
func NewHandler(userService *user.Service, idGen id.Generator) *Handler {
	return &Handler{userService: userService, idGen: idGen}
}

// SetRoleProvider attaches a RoleProvider for dynamic role/policy resolution.
func (h *Handler) SetRoleProvider(rp RoleProvider) {
	h.roleProvider = rp
}

// SetRegistryRepo attaches a RegistryRepository for i/registry/* endpoints.
func (h *Handler) SetRegistryRepo(r repository.RegistryRepository) {
	h.registryRepo = r
}

// RegistryGet handles POST /api/i/registry/get.
func (h *Handler) RegistryGet(c echo.Context) error {
	u := middleware.GetUser(c)
	var req struct {
		Key    string   `json:"key"`
		Scope  []string `json:"scope"`
		Domain *string  `json:"domain"`
	}
	if err := c.Bind(&req); err != nil || req.Key == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "key is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if req.Scope == nil {
		req.Scope = []string{}
	}
	item, err := h.registryRepo.Get(u.ID, req.Key, req.Scope, req.Domain)
	if err != nil {
		return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_KEY", "No such key.", "ac3ed68a-62f0-422b-a7bc-d5e09e8f6a6a"))
	}
	// value をそのまま返す (JSONBの中身)
	return c.JSONBlob(http.StatusOK, item.Value)
}

// RegistrySet handles POST /api/i/registry/set.
func (h *Handler) RegistrySet(c echo.Context) error {
	u := middleware.GetUser(c)
	var req struct {
		Key    string          `json:"key"`
		Value  json.RawMessage `json:"value"`
		Scope  []string        `json:"scope"`
		Domain *string         `json:"domain"`
	}
	if err := c.Bind(&req); err != nil || req.Key == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "key is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if req.Scope == nil {
		req.Scope = []string{}
	}
	item := &model.RegistryItem{
		ID:        h.idGen.Generate(time.Now()),
		UpdatedAt: time.Now(),
		UserID:    u.ID,
		Key:       req.Key,
		Value:     []byte(req.Value),
		Scope:     req.Scope,
		Domain:    req.Domain,
	}
	if err := h.registryRepo.Set(item); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.NoContent(http.StatusNoContent)
}

// RegistryGetAll handles POST /api/i/registry/get-all.
func (h *Handler) RegistryGetAll(c echo.Context) error {
	u := middleware.GetUser(c)
	var req struct {
		Scope  []string `json:"scope"`
		Domain *string  `json:"domain"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "Invalid parameters.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if req.Scope == nil {
		req.Scope = []string{}
	}
	items, err := h.registryRepo.GetAll(u.ID, req.Scope, req.Domain)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	result := make(map[string]json.RawMessage, len(items))
	for _, item := range items {
		result[item.Key] = json.RawMessage(item.Value)
	}
	return c.JSON(http.StatusOK, result)
}

// RegistryKeysWithType handles POST /api/i/registry/keys-with-type.
func (h *Handler) RegistryKeysWithType(c echo.Context) error {
	u := middleware.GetUser(c)
	var req struct {
		Scope  []string `json:"scope"`
		Domain *string  `json:"domain"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "Invalid parameters.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if req.Scope == nil {
		req.Scope = []string{}
	}
	keys, err := h.registryRepo.KeysWithType(u.ID, req.Scope, req.Domain)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.JSON(http.StatusOK, keys)
}

// RegistryRemove handles POST /api/i/registry/remove.
func (h *Handler) RegistryRemove(c echo.Context) error {
	u := middleware.GetUser(c)
	var req struct {
		Key    string   `json:"key"`
		Scope  []string `json:"scope"`
		Domain *string  `json:"domain"`
	}
	if err := c.Bind(&req); err != nil || req.Key == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "key is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if req.Scope == nil {
		req.Scope = []string{}
	}
	if err := h.registryRepo.Remove(u.ID, req.Key, req.Scope, req.Domain); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.NoContent(http.StatusNoContent)
}

// Me handles POST /api/i - returns the authenticated user's info.
func (h *Handler) Me(c echo.Context) error {
	u := middleware.GetUser(c)

	profile := h.userService.GetProfile(u.ID)

	detailed := entity.PackUserDetailed(u, profile)

	// /api/i returns additional private fields
	resp := map[string]any{
		// UserLite fields
		"id":                u.ID,
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
		// UserDetailed fields
		"bannerUrl":      detailed.BannerURL,
		"bannerBlurhash": detailed.BannerBlurhash,
		"isLocked":       detailed.IsLocked,
		"isSilenced":     h.isSilenced(u.ID),
		"isSuspended":    detailed.IsSuspended,
		"description":    detailed.Description,
		"location":       detailed.Location,
		"birthday":       detailed.Birthday,
		"lang":           detailed.Lang,
		"fields":         detailed.Fields,
		"verifiedLinks":  detailed.VerifiedLinks,
		"followersCount": detailed.FollowersCount,
		"followingCount": detailed.FollowingCount,
		"notesCount":     detailed.NotesCount,
		"uri":            detailed.URI,
		"url":            detailed.URL,
		"movedTo":        u.MovedToURI,
		"alsoKnownAs":    u.AlsoKnownAs,
		"updatedAt":      detailed.UpdatedAt,
		"lastFetchedAt":  nil,
		// MeDetailed fields
		"avatarId":            u.AvatarID,
		"bannerId":            u.BannerID,
		"followersVisibility": detailed.FollowersVisibility,
		"followingVisibility": detailed.FollowingVisibility,
		"chatScope":           u.ChatScope,
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
		resp["followedMessage"] = profile.FollowedMessage
		resp["loggedInDays"] = len(profile.LoggedInDates)
		resp["achievements"] = jsonbArray(profile.Achievements)
		resp["securityKeys"] = profile.SecurityKeysAvailable
		// twoFactorBackupCodesStock: full/partial/none
		resp["twoFactorBackupCodesStock"] = backupCodesStock(profile)
		// clientData / room は jsonb を生のまま返すと frontend (本家) が
		// オブジェクトとして parse するため、空/不正時は空オブジェクトに
		// 正規化する。user が手動でキーを書き換えるだけなので scheme は持たない。
		resp["clientData"] = jsonbObject(profile.ClientData)
		resp["room"] = jsonbObject(profile.Room)
	}

	// フロントエンド互換性フィールド (Phase 4.5c / Phase 5)
	isAdmin := false
	isMod := false
	userPolicies := role.DefaultPolicies()
	var userRoles []any
	if h.roleProvider != nil {
		isAdmin = h.roleProvider.IsAdministrator(u.ID)
		isMod = h.roleProvider.IsModerator(u.ID)
		userPolicies = h.roleProvider.GetUserPolicies(u.ID)
		if roles, err := h.roleProvider.GetUserRoles(u.ID); err == nil {
			for _, r := range roles {
				userRoles = append(userRoles, map[string]any{
					"id":              r.ID,
					"name":            r.Name,
					"color":           r.Color,
					"iconUrl":         r.IconURL,
					"description":     r.Description,
					"isModerator":     r.IsModerator,
					"isAdministrator": r.IsAdministrator,
					"displayOrder":    r.DisplayOrder,
				})
			}
		}
	}
	if userRoles == nil {
		userRoles = []any{}
	}
	resp["isAdmin"] = isAdmin
	resp["isModerator"] = isMod
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
	resp["policies"] = userPolicies
	resp["roles"] = userRoles
	// securityKeysList: WebAuthnキーの一覧
	if h.securityKeyRepo != nil {
		if keys, err := h.securityKeyRepo.ListByUser(u.ID); err == nil && len(keys) > 0 {
			list := make([]map[string]any, len(keys))
			for i, k := range keys {
				list[i] = map[string]any{
					"id":       k.ID,
					"name":     k.Name,
					"lastUsed": k.LastUsed.UTC().Format("2006-01-02T15:04:05.000Z"),
				}
			}
			resp["securityKeysList"] = list
		} else {
			resp["securityKeysList"] = []any{}
		}
	} else {
		resp["securityKeysList"] = []any{}
	}
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
	FollowedMessage   *string `json:"followedMessage"`
	PublicReactions   *bool   `json:"publicReactions"`
	IsLocked          *bool   `json:"isLocked"`
	IsBot             *bool   `json:"isBot"`
	IsCat             *bool   `json:"isCat"`
	IsExplorable      *bool   `json:"isExplorable"`
	HideOnlineStatus  *bool   `json:"hideOnlineStatus"`
	AlwaysMarkNsfw    *bool   `json:"alwaysMarkNsfw"`
	AutoSensitive     *bool   `json:"autoSensitive"`
	NoCrawle          *bool   `json:"noCrawle"`
	PreventAiLearning *bool   `json:"preventAiLearning"`
	// Room は frontend の「部屋」機能用の任意スキーマ jsonb。
	// 本家も object をそのまま受け取って上書き保存する (部分マージはしない)。
	Room json.RawMessage `json:"room"`
}

// jsonbObject normalizes a raw jsonb byte slice into map[string]any for the
// Me response. Empty or malformed payloads become an empty object so the
// frontend always sees a stable shape.
func jsonbObject(raw []byte) any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
}

// jsonbArray normalizes a raw jsonb byte slice into []any for the Me response.
// Empty or malformed payloads become an empty array.
func jsonbArray(raw []byte) any {
	if len(raw) == 0 {
		return []any{}
	}
	var out []any
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return []any{}
	}
	return out
}

// backupCodesStock returns "full", "partial", or "none" based on the number of
// remaining 2FA backup codes. Misskey uses 5 codes as the full set.
func backupCodesStock(profile *model.UserProfile) string {
	if !profile.TwoFactorEnabled || len(profile.TwoFactorBackupSecret) == 0 {
		return "none"
	}
	if len(profile.TwoFactorBackupSecret) >= 5 {
		return "full"
	}
	return "partial"
}

// containsProhibitedWord reports whether name contains any entry from words
// case-insensitively (substring match). Empty or whitespace-only entries are
// skipped so a misconfigured empty element cannot ban every display name.
func containsProhibitedWord(name string, words []string) bool {
	lname := strings.ToLower(name)
	for _, w := range words {
		w = strings.TrimSpace(w)
		if w == "" {
			continue
		}
		if strings.Contains(lname, strings.ToLower(w)) {
			return true
		}
	}
	return false
}

// Update handles POST /api/i/update.
func (h *Handler) Update(c echo.Context) error {
	me := middleware.GetUser(c)

	var req UpdateRequest
	if err := c.Bind(&req); err != nil {
		return apierr.JSONInvalidParam(c)
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
		// 表示名の禁止ワードチェック。meta 未注入 / 未設定時は素通りする。
		// 本家 Misskey と同様に部分一致 case-insensitive で、空文字 ("") の
		// クリアリクエストは検査対象外 (ユーザー体験上必要)。
		if h.metaRepo != nil && *req.Name != "" {
			if m, err := h.metaRepo.Fetch(); err == nil && containsProhibitedWord(*req.Name, m.ProhibitedWordsForNameOfUser) {
				return c.JSON(http.StatusBadRequest, apierr.Error("NAME_CONTAINS_PROHIBITED_WORDS", "Your new name contains prohibited words.", "0b3f9f6a-2e7d-4c2c-9d7a-8c6f9b2e1a44"))
			}
		}
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
	if req.FollowedMessage != nil {
		in.FollowedMessage = &req.FollowedMessage
	}
	if req.PublicReactions != nil {
		in.PublicReactions = req.PublicReactions
	}
	if len(req.Room) > 0 {
		// json.RawMessage は親の Unmarshal が構文チェック済みの
		// バイト列を格納する。改変されないよう独自のスライスにコピーする。
		room := append(json.RawMessage(nil), req.Room...)
		in.Room = &room
	}

	bundle, err := h.userService.UpdateProfile(me.ID, in)
	if err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_USER", "No such user.", "4362f8dc-731f-4ad8-a694-be5a88922a24"))
		}
		return apierr.JSONInternalError(c)
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
		return apierr.JSONInvalidParam(c)
	}

	if err := h.userService.PinNote(me.ID, req.NoteID); err != nil {
		switch {
		case errors.Is(err, user.ErrNoteNotFound):
			return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_NOTE", "No such note.", "24fcbfc6-2e37-42b6-8388-c29b32725715"))
		case errors.Is(err, user.ErrAlreadyPinned):
			return c.JSON(http.StatusBadRequest, apierr.Error("ALREADY_PINNED", "That note has already been pinned.", "8b18c2b7-68fe-4edb-9892-c0cbaeb6c913"))
		case errors.Is(err, user.ErrPinLimitExceeded):
			return c.JSON(http.StatusBadRequest, apierr.Error("PIN_LIMIT_EXCEEDED", "You can not pin notes any more.", "72dab508-c64d-498f-8740-a8eec1ba385a"))
		default:
			return apierr.JSONInternalError(c)
		}
	}

	return h.Me(c)
}

// Unpin handles POST /api/i/unpin.
func (h *Handler) Unpin(c echo.Context) error {
	me := middleware.GetUser(c)

	var req PinRequest
	if err := c.Bind(&req); err != nil || req.NoteID == "" {
		return apierr.JSONInvalidParam(c)
	}

	if err := h.userService.UnpinNote(me.ID, req.NoteID); err != nil {
		if errors.Is(err, user.ErrPinNotFound) {
			return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_NOTE", "No such note.", "24fcbfc6-2e37-42b6-8388-c29b32725715"))
		}
		return apierr.JSONInternalError(c)
	}

	return h.Me(c)
}
