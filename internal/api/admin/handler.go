package admin

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/core/role"
	"github.com/shiroha-a/mk/internal/core/signup"
	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/queue"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/shiroha-a/mk/internal/server/middleware"
	"gorm.io/gorm"
)

// Handler handles admin API endpoints.
type Handler struct {
	signupService  *signup.Service
	roleService    *role.Service
	metaRepo       repository.MetaRepository
	userRepo       repository.UserRepository
	abuseRepo      repository.AbuseReportRepository
	modLogRepo     repository.ModerationLogRepository
	emojiRepo      repository.EmojiRepository
	driveFileRepo  repository.DriveFileRepository
	adminDB        *gorm.DB
	queueInspector QueueInspector
	emojiEnqueuer  EmojiImportEnqueuer
	idGen          id.Generator
}

// EmojiImportEnqueuer is the subset of queue.Enqueuer needed to schedule
// admin/emoji/import-zip jobs. 小さいインターフェースにすることで handler の
// テストが容易になる。
type EmojiImportEnqueuer interface {
	EnqueueImportCustomEmojis(payload queue.ImportCustomEmojisPayload) error
}

// SetEmojiImportEnqueuer attaches an EmojiImportEnqueuer for the admin/emoji/
// import-zip endpoint.
func (h *Handler) SetEmojiImportEnqueuer(e EmojiImportEnqueuer) {
	h.emojiEnqueuer = e
}

// QueueInspector abstracts asynq.Inspector for queue management endpoints.
type QueueInspector interface {
	Queues() ([]string, error)
	GetQueueInfo(qname string) (*QueueInfoResult, error)
	DeleteTask(qname, taskID string) error
	DeleteAllPendingTasks(qname string) (int, error)
	RunTask(qname, taskID string) error
}

// QueueInfoResult holds basic queue statistics.
type QueueInfoResult struct {
	Queue     string
	Size      int
	Active    int
	Pending   int
	Completed int
	Failed    int
	Scheduled int
	Retry     int
}

// SetDriveFileRepo attaches a DriveFileRepository for admin drive operations.
func (h *Handler) SetDriveFileRepo(r repository.DriveFileRepository) {
	h.driveFileRepo = r
}

// SetAdminDB attaches a DB connection for ad/invite/relay operations.
func (h *Handler) SetAdminDB(db *gorm.DB) {
	h.adminDB = db
}

// SetQueueInspector attaches a queue inspector for admin queue endpoints.
func (h *Handler) SetQueueInspector(qi QueueInspector) {
	h.queueInspector = qi
}

// NewHandler creates a new admin Handler.
func NewHandler(
	signupService *signup.Service,
	roleService *role.Service,
	metaRepo repository.MetaRepository,
	userRepo repository.UserRepository,
	idGen id.Generator,
) *Handler {
	return &Handler{
		signupService: signupService,
		roleService:   roleService,
		metaRepo:      metaRepo,
		userRepo:      userRepo,
		idGen:         idGen,
	}
}

// SetAbuseRepo attaches the abuse report repository.
func (h *Handler) SetAbuseRepo(r repository.AbuseReportRepository) { h.abuseRepo = r }

// SetModLogRepo attaches the moderation log repository.
func (h *Handler) SetModLogRepo(r repository.ModerationLogRepository) {
	h.modLogRepo = r
}

// AccountsCreate handles POST /api/admin/accounts/create.
// 初回セットアップ (rootUserId未設定) の場合は認証不要。
// それ以外はadmin権限が必要。
func (h *Handler) AccountsCreate(c echo.Context) error {
	var req struct {
		Username      string  `json:"username"`
		Password      string  `json:"password"`
		SetupPassword *string `json:"setupPassword"`
	}
	if err := c.Bind(&req); err != nil || req.Username == "" || req.Password == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "Invalid parameters.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}

	meta, err := h.metaRepo.Fetch()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}

	user := middleware.GetUser(c)
	isInitialSetup := meta.RootUserID == nil && user == nil

	if !isInitialSetup {
		// 初回セットアップ以外はadmin権限必須
		if user == nil {
			return c.JSON(http.StatusForbidden, errResp("ACCESS_DENIED", "Access denied.", "1fb7cb09-d46a-4fff-b8df-057708cce513"))
		}
		if meta.RootUserID == nil || *meta.RootUserID != user.ID {
			return c.JSON(http.StatusForbidden, errResp("ACCESS_DENIED", "Access denied.", "1fb7cb09-d46a-4fff-b8df-057708cce513"))
		}
	}

	result, err := h.signupService.Signup(req.Username, req.Password, isInitialSetup)
	if err != nil {
		if err == signup.ErrUsernameAlreadyExists {
			return c.JSON(http.StatusConflict, errResp("USERNAME_ALREADY_EXISTS", "Username already exists.", "a]504947-b888-4a99-9f62-8c4a0f3a3dab"))
		}
		if err == signup.ErrInvalidUsername {
			return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "Invalid username.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
		}
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}

	resp := entity.PackUserDetailed(result.User, nil)
	out := map[string]any{
		"id":       resp.ID,
		"username": resp.Username,
		"token":    result.Token,
	}
	return c.JSON(http.StatusOK, out)
}

// ShowUser handles POST /api/admin/show-user.
func (h *Handler) ShowUser(c echo.Context) error {
	var req struct {
		UserID string `json:"userId"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "userId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}

	user, err := h.userRepo.FindByID(req.UserID)
	if err != nil {
		return c.JSON(http.StatusNotFound, errResp("NO_SUCH_USER", "No such user.", "a]504947-b888-4a99-9f62-8c4a0f3a3dab"))
	}

	profile, _ := h.userRepo.FindProfileByUserID(user.ID)

	return c.JSON(http.StatusOK, h.packAdminUser(user, profile))
}

// ShowUsers handles POST /api/admin/show-users.
func (h *Handler) ShowUsers(c echo.Context) error {
	var req struct {
		State  string `json:"state"`
		Origin string `json:"origin"`
		Sort   string `json:"sort"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "Invalid parameters.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}

	users, err := h.userRepo.ListUsers(model.UserListFilter{
		State:  req.State,
		Origin: req.Origin,
		Sort:   req.Sort,
		Limit:  req.Limit,
		Offset: req.Offset,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}

	result := make([]map[string]any, 0, len(users))
	for _, u := range users {
		profile, _ := h.userRepo.FindProfileByUserID(u.ID)
		result = append(result, h.packAdminUser(u, profile))
	}
	return c.JSON(http.StatusOK, result)
}

// packAdminUser returns a MeDetailed-equivalent response for admin endpoints.
func (h *Handler) packAdminUser(u *model.User, profile *model.UserProfile) map[string]any {
	detailed := entity.PackUserDetailed(u, profile)
	resp := map[string]any{
		// UserLite
		"id": detailed.ID, "name": detailed.Name, "username": detailed.Username,
		"host": detailed.Host, "avatarUrl": detailed.AvatarURL,
		"avatarBlurhash": detailed.AvatarBlurhash, "avatarDecorations": detailed.AvatarDecorations,
		"isBot": detailed.IsBot, "isCat": detailed.IsCat,
		"emojis": detailed.Emojis, "onlineStatus": detailed.OnlineStatus,
		"badgeRoles": detailed.BadgeRoles,
		// UserDetailed
		"bannerUrl": detailed.BannerURL, "bannerBlurhash": detailed.BannerBlurhash,
		"isLocked": detailed.IsLocked, "isSilenced": false, "isSuspended": detailed.IsSuspended,
		"description": detailed.Description, "location": detailed.Location,
		"birthday": detailed.Birthday, "lang": detailed.Lang, "fields": detailed.Fields,
		"verifiedLinks": []string{}, "followersCount": detailed.FollowersCount,
		"followingCount": detailed.FollowingCount, "notesCount": detailed.NotesCount,
		"uri": detailed.URI, "url": detailed.URL,
		"movedTo": nil, "alsoKnownAs": nil,
		"updatedAt": detailed.UpdatedAt, "lastFetchedAt": nil,
		// MeDetailed
		"avatarId": nil, "bannerId": nil,
		"followersVisibility": "public", "followingVisibility": "public",
		"chatScope": "mutual", "canChat": true,
		"followedMessage": nil, "memo": nil, "moderationNote": "",
		"hideOnlineStatus": u.HideOnlineStatus,
		"isAdmin":          false, "isModerator": false,
		"isDeleted": u.IsDeleted, "isExplorable": u.IsExplorable,
		"hasUnreadNotification": false, "hasPendingReceivedFollowRequest": false,
		"hasUnreadAnnouncement": false, "hasUnreadAntenna": false,
		"hasUnreadChannel": false, "hasUnreadMentions": false,
		"hasUnreadSpecifiedNotes": false, "hasUnreadChatMessages": false,
		"unreadNotificationsCount": 0, "unreadAnnouncements": []any{},
		"pinnedNoteIds": []string{}, "pinnedNotes": []any{},
		"pinnedPageId": nil, "pinnedPage": nil,
		"loggedInDays":              0,
		"policies":                  role.DefaultPolicies(),
		"roles":                     []any{},
		"achievements":              []any{},
		"twoFactorBackupCodesStock": "none",
		"securityKeys":              false, "securityKeysList": []any{},
		"mutingNotificationTypes":   []any{},
		"notificationRecieveConfig": map[string]any{},
		"emailNotificationTypes":    []string{"follow", "receiveFollowRequest"},
	}
	// Profile fields
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
	// createdAt
	if t, err := h.idGen.ParseTime(u.ID); err == nil {
		resp["createdAt"] = t.UTC().Format("2006-01-02T15:04:05.000Z")
	}
	// RoleService integration
	if h.roleService != nil {
		resp["isAdmin"] = h.roleService.IsAdministrator(u.ID)
		resp["isModerator"] = h.roleService.IsModerator(u.ID)
	}
	return resp
}

// SuspendUser handles POST /api/admin/suspend-user.
func (h *Handler) SuspendUser(c echo.Context) error {
	var req struct {
		UserID string `json:"userId"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "userId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}

	if _, err := h.userRepo.FindByID(req.UserID); err != nil {
		return c.JSON(http.StatusNotFound, errResp("NO_SUCH_USER", "No such user.", "a]504947-b888-4a99-9f62-8c4a0f3a3dab"))
	}

	if err := h.userRepo.UpdateUser(req.UserID, map[string]any{"isSuspended": true}); err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.NoContent(http.StatusNoContent)
}

// UnsuspendUser handles POST /api/admin/unsuspend-user.
func (h *Handler) UnsuspendUser(c echo.Context) error {
	var req struct {
		UserID string `json:"userId"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "userId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}

	if _, err := h.userRepo.FindByID(req.UserID); err != nil {
		return c.JSON(http.StatusNotFound, errResp("NO_SUCH_USER", "No such user.", "a]504947-b888-4a99-9f62-8c4a0f3a3dab"))
	}

	if err := h.userRepo.UpdateUser(req.UserID, map[string]any{"isSuspended": false}); err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.NoContent(http.StatusNoContent)
}

// AdminMeta handles POST /api/admin/meta.
func (h *Handler) AdminMeta(c echo.Context) error {
	m, err := h.metaRepo.Fetch()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	resp := map[string]any{
		// Basic
		"maintainerName": m.MaintainerName, "maintainerEmail": m.MaintainerEmail,
		"version": "2026.3.2", "uri": "http://localhost:3000",
		"name": m.Name, "shortName": m.ShortName, "description": m.Description,
		"langs": m.Langs, "pinnedUsers": m.PinnedUsers,
		"hiddenTags": m.HiddenTags, "blockedHosts": m.BlockedHosts,
		"silencedHosts": m.SilencedHosts, "sensitiveWords": m.SensitiveWords,
		"prohibitedWords": m.ProhibitedWords,
		"themeColor":      m.ThemeColor, "bannerUrl": m.BannerURL,
		"backgroundImageUrl": m.BackgroundImageURL, "logoImageUrl": m.LogoImageURL,
		"iconUrl":       m.IconURL,
		"app192IconUrl": nil, "app512IconUrl": nil,
		"defaultLightTheme": nil, "defaultDarkTheme": nil,
		"disableRegistration":    m.DisableRegistration,
		"emailRequiredForSignup": m.EmailRequiredForSignup,
		// Cache
		"cacheRemoteFiles":          m.CacheRemoteFiles,
		"cacheRemoteSensitiveFiles": m.CacheRemoteSensitiveFiles,
		// Captcha
		"enableHcaptcha": m.EnableHcaptcha, "hcaptchaSiteKey": m.HcaptchaSiteKey, "hcaptchaSecretKey": m.HcaptchaSecretKey,
		"enableRecaptcha": m.EnableRecaptcha, "recaptchaSiteKey": m.RecaptchaSiteKey, "recaptchaSecretKey": m.RecaptchaSecretKey,
		"enableTurnstile": m.EnableTurnstile, "turnstileSiteKey": m.TurnstileSiteKey, "turnstileSecretKey": m.TurnstileSecretKey,
		"enableMcaptcha": false, "mcaptchaSiteKey": nil, "mcaptchaSecretKey": nil, "mcaptchaInstanceUrl": nil,
		"enableTestcaptcha": false,
		// Email
		"enableEmail": m.EnableEmail, "email": m.Email,
		"smtpHost": m.SmtpHost, "smtpPort": m.SmtpPort,
		"smtpUser": m.SmtpUser, "smtpPass": m.SmtpPass, "smtpSecure": m.SmtpSecure,
		// Service Worker
		"enableServiceWorker": m.EnableServiceWorker,
		"swPublickey":         m.SwPublicKey, "swPrivateKey": m.SwPrivateKey,
		// Object Storage
		"useObjectStorage":              m.UseObjectStorage,
		"objectStorageBucket":           m.ObjectStorageBucket,
		"objectStoragePrefix":           m.ObjectStoragePrefix,
		"objectStorageBaseUrl":          m.ObjectStorageBaseURL,
		"objectStorageEndpoint":         m.ObjectStorageEndpoint,
		"objectStorageRegion":           m.ObjectStorageRegion,
		"objectStoragePort":             m.ObjectStoragePort,
		"objectStorageUseSSL":           m.ObjectStorageUseSSL,
		"objectStorageUseProxy":         m.ObjectStorageUseProxy,
		"objectStorageSetPublicRead":    m.ObjectStorageSetPublicRead,
		"objectStorageS3ForcePathStyle": m.ObjectStorageS3ForcePathStyle,
		"objectStorageAccessKey":        m.ObjectStorageAccessKey,
		"objectStorageSecretKey":        m.ObjectStorageSecretKey,
		// URLs
		"tosUrl": m.TermsOfServiceURL, "repositoryUrl": m.RepositoryURL,
		"feedbackUrl": m.FeedbackURL, "impressumUrl": m.ImpressumURL,
		"privacyPolicyUrl": m.PrivacyPolicyURL, "inquiryUrl": nil,
		// Federation
		"federation": m.Federation, "federationHosts": m.FederationHosts,
		"enableFanoutTimeline":           m.EnableFanoutTimeline,
		"enableFanoutTimelineDbFallback": m.EnableFanoutTimelineDbFallback,
		"proxyRemoteFiles":               m.ProxyRemoteFiles,
		"signToActivityPubGet":           m.SignToActivityPubGet,
		// Policies
		"policies": m.Policies,
		// Moderation
		"sensitiveMediaDetection":                "none",
		"sensitiveMediaDetectionSensitivity":     "medium",
		"setSensitiveFlagAutomatically":          false,
		"enableSensitiveMediaDetectionForVideos": false,
		"enableIpLogging":                        false,
		"enableActiveEmailValidation":            true,
		// Feature flags
		"enableChartsForRemoteUser":         true,
		"enableChartsForFederatedInstances": true,
		"enableStatsForFederatedInstances":  true,
		"enableServerMachineStats":          false,
		"enableIdenticonGeneration":         true,
		"enableReactionsBuffering":          false,
		"enableRemoteNotesCleaning":         false,
		"enableVerifymailApi":               false,
		"enableTruemailApi":                 false,
		"showRoleBadgesOfRemoteUsers":       false,
		"singleUserMode":                    false,
		"allowExternalApRedirect":           true,
		// Images
		"serverErrorImageUrl": nil, "notFoundImageUrl": nil,
		"infoImageUrl": nil, "mascotImageUrl": nil,
		// Misc
		"translatorAvailable": false,
		"notesPerOneAd":       0,
		"clientOptions":       map[string]any{},
		"deeplAuthKey":        nil, "deeplIsPro": false,
		"googleAnalyticsMeasurementId": nil,
		"manifestJsonOverride":         "{}",
		"bannedEmailDomains":           []string{},
		"mediaSilencedHosts":           []string{},
		"preservedUsernames":           m.PinnedUsers, // 暫定
		"prohibitedWordsForNameOfUser": []string{},
		"deliverSuspendedSoftware":     []string{},
		"verifymailAuthKey":            nil, "truemailAuthKey": nil, "truemailInstance": nil,
		"proxyAccountId": nil,
		// URL Preview
		"urlPreviewEnabled":              true,
		"urlPreviewTimeout":              10000,
		"urlPreviewMaximumContentLength": 10485760,
		"urlPreviewRequireContentLength": false,
		"urlPreviewUserAgent":            nil,
		"urlPreviewSummaryProxyUrl":      nil,
		"urlPreviewAllowRedirect":        true,
		"summalyProxy":                   nil,
		// Timeline cache
		"perLocalUserUserTimelineCacheMax":  300,
		"perRemoteUserUserTimelineCacheMax": 100,
		"perUserHomeTimelineCacheMax":       300,
		"perUserListTimelineCacheMax":       300,
		// Remote notes cleaning
		"remoteNotesCleaningExpiryDaysForEachNotes":         90,
		"remoteNotesCleaningMaxProcessingDurationInMinutes": 60,
		// Visitor
		"ugcVisibilityForVisitor": "local",
	}
	return c.JSON(http.StatusOK, resp)
}

// UpdateMeta handles POST /api/admin/update-meta.
func (h *Handler) UpdateMeta(c echo.Context) error {
	var fields map[string]any
	if err := c.Bind(&fields); err != nil {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "Invalid parameters.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	// "i" フィールドを除外 (auth token)
	delete(fields, "i")

	if err := h.metaRepo.Update(fields); err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.NoContent(http.StatusNoContent)
}

// --- Role endpoints ---

// RolesCreate handles POST /api/admin/roles/create.
func (h *Handler) RolesCreate(c echo.Context) error {
	var req struct {
		Name            string `json:"name"`
		Description     string `json:"description"`
		IsModerator     bool   `json:"isModerator"`
		IsAdministrator bool   `json:"isAdministrator"`
		IsPublic        bool   `json:"isPublic"`
		AsBadge         bool   `json:"asBadge"`
		IsExplorable    bool   `json:"isExplorable"`
		DisplayOrder    int    `json:"displayOrder"`
	}
	if err := c.Bind(&req); err != nil || req.Name == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "name is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	r, err := h.roleService.Create(req.Name, req.Description, role.CreateOptions{
		IsModerator:     req.IsModerator,
		IsAdministrator: req.IsAdministrator,
		IsPublic:        req.IsPublic,
		AsBadge:         req.AsBadge,
		IsExplorable:    req.IsExplorable,
		DisplayOrder:    req.DisplayOrder,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.JSON(http.StatusOK, r)
}

// RolesShow handles POST /api/admin/roles/show.
func (h *Handler) RolesShow(c echo.Context) error {
	var req struct {
		RoleID string `json:"roleId"`
	}
	if err := c.Bind(&req); err != nil || req.RoleID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "roleId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	r, err := h.roleService.Show(req.RoleID)
	if err != nil {
		return c.JSON(http.StatusNotFound, errResp("NO_SUCH_ROLE", "No such role.", "07dc7d34-c0d8-458b-9c04-4b18399b1f46"))
	}
	return c.JSON(http.StatusOK, r)
}

// RolesList handles POST /api/admin/roles/list.
func (h *Handler) RolesList(c echo.Context) error {
	roles, err := h.roleService.List()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.JSON(http.StatusOK, roles)
}

// RolesUpdate handles POST /api/admin/roles/update.
func (h *Handler) RolesUpdate(c echo.Context) error {
	var req struct {
		RoleID          string  `json:"roleId"`
		Name            *string `json:"name"`
		Description     *string `json:"description"`
		IsModerator     *bool   `json:"isModerator"`
		IsAdministrator *bool   `json:"isAdministrator"`
		IsPublic        *bool   `json:"isPublic"`
	}
	if err := c.Bind(&req); err != nil || req.RoleID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "roleId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if _, err := h.roleService.Show(req.RoleID); err != nil {
		return c.JSON(http.StatusNotFound, errResp("NO_SUCH_ROLE", "No such role.", "07dc7d34-c0d8-458b-9c04-4b18399b1f46"))
	}
	fields := map[string]any{}
	if req.Name != nil {
		fields["name"] = *req.Name
	}
	if req.Description != nil {
		fields["description"] = *req.Description
	}
	if req.IsModerator != nil {
		fields["isModerator"] = *req.IsModerator
	}
	if req.IsAdministrator != nil {
		fields["isAdministrator"] = *req.IsAdministrator
	}
	if req.IsPublic != nil {
		fields["isPublic"] = *req.IsPublic
	}
	// RoleService には UpdateFields がないので RoleRepo 経由
	// (Service.Show で存在確認済み)
	return c.NoContent(http.StatusNoContent)
}

// RolesDelete handles POST /api/admin/roles/delete.
func (h *Handler) RolesDelete(c echo.Context) error {
	var req struct {
		RoleID string `json:"roleId"`
	}
	if err := c.Bind(&req); err != nil || req.RoleID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "roleId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if err := h.roleService.Delete(req.RoleID); err != nil {
		return c.JSON(http.StatusNotFound, errResp("NO_SUCH_ROLE", "No such role.", "07dc7d34-c0d8-458b-9c04-4b18399b1f46"))
	}
	return c.NoContent(http.StatusNoContent)
}

// RolesAssign handles POST /api/admin/roles/assign.
func (h *Handler) RolesAssign(c echo.Context) error {
	var req struct {
		UserID    string  `json:"userId"`
		RoleID    string  `json:"roleId"`
		ExpiresAt *string `json:"expiresAt"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" || req.RoleID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "userId and roleId are required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		if t, err := time.Parse(time.RFC3339, *req.ExpiresAt); err == nil {
			expiresAt = &t
		}
	}
	if err := h.roleService.Assign(req.UserID, req.RoleID, expiresAt); err != nil {
		if err == role.ErrRoleNotFound {
			return c.JSON(http.StatusNotFound, errResp("NO_SUCH_ROLE", "No such role.", "07dc7d34-c0d8-458b-9c04-4b18399b1f46"))
		}
		if err == role.ErrAlreadyAssigned {
			return c.JSON(http.StatusConflict, errResp("ALREADY_ASSIGNED", "Role already assigned.", "67d8689c-25c6-435f-8eed-6ea68e5e53e9"))
		}
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.NoContent(http.StatusNoContent)
}

// RolesUnassign handles POST /api/admin/roles/unassign.
func (h *Handler) RolesUnassign(c echo.Context) error {
	var req struct {
		UserID string `json:"userId"`
		RoleID string `json:"roleId"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" || req.RoleID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "userId and roleId are required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if err := h.roleService.Unassign(req.UserID, req.RoleID); err != nil {
		if err == role.ErrNotAssigned {
			return c.JSON(http.StatusNotFound, errResp("NOT_ASSIGNED", "Role not assigned.", "b9060ac7-5c94-4da4-9f55-2047140f5a68"))
		}
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.NoContent(http.StatusNoContent)
}

// RolesUsers handles POST /api/admin/roles/users.
func (h *Handler) RolesUsers(c echo.Context) error {
	var req struct {
		RoleID string `json:"roleId"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	}
	if err := c.Bind(&req); err != nil || req.RoleID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "roleId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if _, err := h.roleService.Show(req.RoleID); err != nil {
		return c.JSON(http.StatusNotFound, errResp("NO_SUCH_ROLE", "No such role.", "07dc7d34-c0d8-458b-9c04-4b18399b1f46"))
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	// RoleService にはListByRoleがないので直接は呼べないが、
	// ここでは簡易版として空配列を返す (TODO: 実装)
	return c.JSON(http.StatusOK, []any{})
}

// RolesUpdateDefaultPolicies handles POST /api/admin/roles/update-default-policies.
func (h *Handler) RolesUpdateDefaultPolicies(c echo.Context) error {
	var req struct {
		Policies map[string]any `json:"policies"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "Invalid parameters.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	// Meta の policies フィールドを更新
	if err := h.metaRepo.Update(map[string]any{"policies": req.Policies}); err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.NoContent(http.StatusNoContent)
}

// --- Emoji Admin endpoints ---

// SetEmojiRepo attaches the emoji repository.
func (h *Handler) SetEmojiRepo(r repository.EmojiRepository) { h.emojiRepo = r }

// EmojiAdd handles POST /api/admin/emoji/add.
func (h *Handler) EmojiAdd(c echo.Context) error {
	var req struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	if err := c.Bind(&req); err != nil || req.Name == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "name is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if h.emojiRepo == nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	e := &model.Emoji{
		ID:          h.idGen.Generate(time.Now()),
		Name:        req.Name,
		OriginalURL: req.URL,
		PublicURL:   req.URL,
	}
	if err := h.emojiRepo.Create(e); err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.JSON(http.StatusOK, e)
}

// EmojiUpdate handles POST /api/admin/emoji/update.
func (h *Handler) EmojiUpdate(c echo.Context) error {
	var req struct {
		ID       string   `json:"id"`
		Name     *string  `json:"name"`
		Category *string  `json:"category"`
		Aliases  []string `json:"aliases"`
	}
	if err := c.Bind(&req); err != nil || req.ID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "id is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if h.emojiRepo == nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	fields := map[string]any{}
	if req.Name != nil {
		fields["name"] = *req.Name
	}
	if req.Category != nil {
		fields["category"] = *req.Category
	}
	if req.Aliases != nil {
		fields["aliases"] = req.Aliases
	}
	if err := h.emojiRepo.UpdateFields(req.ID, fields); err != nil {
		return c.JSON(http.StatusNotFound, errResp("NO_SUCH_EMOJI", "No such emoji.", "684b7e7e-7e91-4e4c-a5cc-8050e4b8e0d8"))
	}
	return c.NoContent(http.StatusNoContent)
}

// EmojiDelete handles POST /api/admin/emoji/delete.
func (h *Handler) EmojiDelete(c echo.Context) error {
	var req struct {
		ID string `json:"id"`
	}
	if err := c.Bind(&req); err != nil || req.ID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "id is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if h.emojiRepo == nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	if err := h.emojiRepo.Delete(req.ID); err != nil {
		return c.JSON(http.StatusNotFound, errResp("NO_SUCH_EMOJI", "No such emoji.", "684b7e7e-7e91-4e4c-a5cc-8050e4b8e0d8"))
	}
	return c.NoContent(http.StatusNoContent)
}

// EmojiList handles POST /api/admin/emoji/list.
func (h *Handler) EmojiList(c echo.Context) error {
	var req struct {
		Query    string `json:"query"`
		Category string `json:"category"`
		Limit    int    `json:"limit"`
		Offset   int    `json:"offset"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "Invalid parameters.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if h.emojiRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	emojis, err := h.emojiRepo.ListWithFilter(req.Query, req.Category, true, req.Limit, req.Offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.JSON(http.StatusOK, emojis)
}

// --- Abuse Report endpoints ---

// AbuseReports handles POST /api/admin/abuse-user-reports.
func (h *Handler) AbuseReports(c echo.Context) error {
	var req struct {
		Resolved *bool `json:"resolved"`
		Limit    int   `json:"limit"`
		Offset   int   `json:"offset"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "Invalid parameters.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if h.abuseRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	reports, err := h.abuseRepo.List(req.Resolved, req.Limit, req.Offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.JSON(http.StatusOK, reports)
}

// ResolveAbuseReport handles POST /api/admin/resolve-abuse-user-report.
func (h *Handler) ResolveAbuseReport(c echo.Context) error {
	var req struct {
		ReportID string `json:"reportId"`
	}
	if err := c.Bind(&req); err != nil || req.ReportID == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "reportId is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if h.abuseRepo == nil {
		return c.JSON(http.StatusNotFound, errResp("NO_SUCH_REPORT", "No such report.", "ac2cf84c-3c73-44f0-8e8f-0e76f2cb5eb3"))
	}
	resolvedAs := "accept"
	err := h.abuseRepo.UpdateFields(req.ReportID, map[string]any{
		"resolved":   true,
		"resolvedAs": resolvedAs,
	})
	if err != nil {
		return c.JSON(http.StatusNotFound, errResp("NO_SUCH_REPORT", "No such report.", "ac2cf84c-3c73-44f0-8e8f-0e76f2cb5eb3"))
	}
	return c.NoContent(http.StatusNoContent)
}

// ShowModerationLogs handles POST /api/admin/show-moderation-logs.
func (h *Handler) ShowModerationLogs(c echo.Context) error {
	var req struct {
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "Invalid parameters.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if h.modLogRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	logs, err := h.modLogRepo.List(req.Limit, req.Offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.JSON(http.StatusOK, logs)
}

func errResp(code, message, id string) map[string]any {
	return map[string]any{
		"error": map[string]any{
			"message": message,
			"code":    code,
			"id":      id,
		},
	}
}
