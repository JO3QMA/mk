package signup

import (
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/shiroha-a/mk/internal/api/apierr"
	"github.com/shiroha-a/mk/internal/core/captcha"
	"github.com/shiroha-a/mk/internal/core/role"
	coresignup "github.com/shiroha-a/mk/internal/core/signup"
	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

// TicketStore abstracts registration_ticket DB operations for testability.
type TicketStore interface {
	FindByCode(code string) (*model.RegistrationTicket, error)
	MarkUsed(ticketID, userID string) error
}

// Handler handles POST /api/signup.
type Handler struct {
	signupService *coresignup.Service
	metaRepo      repository.MetaRepository
	idGen         id.Generator
	captchaSvc    *captcha.Service // optional
	ticketStore   TicketStore      // optional, invitation code検証用
	testMode      bool             // true のとき disableRegistration / captcha をバイパス (本家 TS と同じ)
}

// SetTestMode enables test-mode bypass (本家 `process.env.NODE_ENV !== 'test'` 相当).
func (h *Handler) SetTestMode(v bool) {
	h.testMode = v
}

// NewHandler creates a new signup Handler.
func NewHandler(signupService *coresignup.Service, metaRepo repository.MetaRepository, idGen id.Generator) *Handler {
	return &Handler{signupService: signupService, metaRepo: metaRepo, idGen: idGen}
}

// SetCaptcha attaches a CaptchaService for signup verification.
func (h *Handler) SetCaptcha(svc *captcha.Service) {
	h.captchaSvc = svc
}

// SetTicketStore attaches a TicketStore for invitation code validation.
func (h *Handler) SetTicketStore(ts TicketStore) {
	h.ticketStore = ts
}

type signupRequest struct {
	Username       string `json:"username"`
	Password       string `json:"password"`
	InvitationCode string `json:"invitationCode"`
	// CAPTCHA tokens (フィールド名はTS版スキーマに準拠)
	HcaptchaResponse    string `json:"hcaptcha-response"`
	RecaptchaResponse   string `json:"g-recaptcha-response"`
	TurnstileResponse   string `json:"turnstile-response"`
	McaptchaResponse    string `json:"m-captcha-response"`
	TestcaptchaResponse string `json:"testcaptcha-response"`
}

// Signup handles POST /api/signup.
func (h *Handler) Signup(c echo.Context) error {
	var req signupRequest
	if err := c.Bind(&req); err != nil || req.Username == "" || req.Password == "" {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "Invalid parameters.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}

	meta, err := h.metaRepo.Fetch()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}

	// メール認証フローは未実装 (テストモードではスキップ)
	if !h.testMode && meta.EmailRequiredForSignup {
		return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "Email-required signup is not yet supported.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}

	// 登録無効時はinvitation code必須 (テストモードではバイパス — 本家 TS 互換)
	var ticket *model.RegistrationTicket
	if !h.testMode && meta.DisableRegistration {
		t, vErr := h.validateInvitationCode(req.InvitationCode)
		if vErr != nil {
			return c.JSON(http.StatusBadRequest, errResp("INVITATION_CODE_INVALID", "Invalid invitation code.", "11e71a03-43c4-4a99-92cf-bb7e2c581998"))
		}
		ticket = t
	}

	// CAPTCHA検証 (テストモードではスキップ)
	if !h.testMode && h.captchaSvc != nil {
		tokens := captcha.CaptchaTokens{
			Hcaptcha:    req.HcaptchaResponse,
			Recaptcha:   req.RecaptchaResponse,
			Turnstile:   req.TurnstileResponse,
			Mcaptcha:    req.McaptchaResponse,
			Testcaptcha: req.TestcaptchaResponse,
		}
		if err := h.captchaSvc.Verify(c.Request().Context(), tokens); err != nil {
			return c.JSON(http.StatusBadRequest, errResp("CAPTCHA_FAILED", "Captcha verification failed.", "bdc32ef5-b0f4-40c0-b767-673b2e3e1f5a"))
		}
	}

	result, err := h.signupService.Signup(req.Username, req.Password, false)
	if err != nil {
		if err == coresignup.ErrUsernameAlreadyExists {
			return c.JSON(http.StatusConflict, errResp("USERNAME_ALREADY_EXISTS", "Username already exists.", "a504947-b888-4a99-9f62-8c4a0f3a3dab"))
		}
		if err == coresignup.ErrInvalidUsername {
			return c.JSON(http.StatusBadRequest, errResp("INVALID_PARAM", "Invalid username.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
		}
		if err == coresignup.ErrUsernameReserved {
			return c.JSON(http.StatusBadRequest, errResp("USED_USERNAME", "That username is reserved.", "4b54bee6-2c25-42c3-a10f-7d0d1fbd91f9"))
		}
		return c.JSON(http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}

	// invitation code使用済みにする
	if ticket != nil && h.ticketStore != nil {
		_ = h.ticketStore.MarkUsed(ticket.ID, result.User.ID)
	}

	return c.JSON(http.StatusOK, packSignupResponse(result.User, result.Token, h.idGen))
}

// validateInvitationCode checks the ticket store for a valid invitation code.
func (h *Handler) validateInvitationCode(code string) (*model.RegistrationTicket, error) {
	if code == "" || h.ticketStore == nil {
		return nil, errInvalidCode
	}
	ticket, err := h.ticketStore.FindByCode(code)
	if err != nil {
		return nil, errInvalidCode
	}
	if ticket.UsedByID != nil {
		return nil, errInvalidCode
	}
	if ticket.ExpiresAt != nil && ticket.ExpiresAt.Before(time.Now()) {
		return nil, errInvalidCode
	}
	return ticket, nil
}

// packSignupResponse builds a MeDetailed + token response for a newly created user.
func packSignupResponse(u *model.User, token string, idGen id.Generator) map[string]any {
	detailed := entity.PackUserDetailed(u, nil, idGen)
	return map[string]any{
		// UserLite
		"id":                detailed.ID,
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
		// UserDetailed
		"bannerUrl":      detailed.BannerURL,
		"bannerBlurhash": detailed.BannerBlurhash,
		"isLocked":       detailed.IsLocked,
		"isSilenced":     false,
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
		"pinnedNoteIds":  detailed.PinnedNoteIDs,
		"pinnedNotes":    detailed.PinnedNotes,
		"roles":          detailed.Roles,
		"uri":            detailed.URI,
		"url":            detailed.URL,
		"movedTo":        nil,
		"alsoKnownAs":    nil,
		"createdAt":      detailed.CreatedAt,
		"updatedAt":      detailed.UpdatedAt,
		"lastFetchedAt":  nil,
		// MeDetailed (新規ユーザーのデフォルト値)
		"avatarId":                 nil,
		"bannerId":                 nil,
		"followersVisibility":      "public",
		"followingVisibility":      "public",
		"chatScope":                "mutual",
		"canChat":                  true,
		"followedMessage":          nil,
		"memo":                     nil,
		"moderationNote":           nil,
		"isAdmin":                  false,
		"isModerator":              false,
		"hideOnlineStatus":         u.HideOnlineStatus,
		"email":                    nil,
		"emailVerified":            false,
		"autoAcceptFollowed":       true,
		"noCrawle":                 false,
		"preventAiLearning":        true,
		"alwaysMarkNsfw":           false,
		"autoSensitive":            false,
		"carefulBot":               false,
		"injectFeaturedNote":       true,
		"receiveAnnouncementEmail": true,
		"twoFactorEnabled":         false,
		"usePasswordLessLogin":     false,
		"publicReactions":          true,
		"mutedWords":               []any{},
		"hardMutedWords":           []any{},
		"mutedInstances":           []any{},
		"policies":                 role.DefaultPolicies(),
		"token":                    token,
	}
}

// defaultPolicies returns the Misskey default policies for a new user.
var errInvalidCode = errors.New("invalid invitation code")

func errResp(code, message, id string) map[string]any {
	return apierr.Error(code, message, id)
}
