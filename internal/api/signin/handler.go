package signin

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
	"github.com/shiroha-a/mk/internal/core/captcha"
	"github.com/shiroha-a/mk/internal/core/twofactor"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/datatypes"
)

// IPLogger records user IPs on successful authentication.
type IPLogger interface {
	Upsert(userID, ip string) error
}

// Handler handles signin-related API endpoints.
type Handler struct {
	userRepo        repository.UserRepository
	webauthnSvc     *twofactor.WebAuthnService
	securityKeyRepo repository.UserSecurityKeyRepository
	captchaSvc      *captcha.Service
	ipLogger        IPLogger
	ipLoggingOn     bool
	signinRepo      repository.SigninRepository
	idGen           id.Generator
}

// SetIPLogger attaches an IPLogger and enables IP logging.
func (h *Handler) SetIPLogger(logger IPLogger, enabled bool) {
	h.ipLogger = logger
	h.ipLoggingOn = enabled
}

// NewHandler creates a new signin Handler.
func NewHandler(userRepo repository.UserRepository) *Handler {
	return &Handler{userRepo: userRepo}
}

// SetSigninRepo attaches a SigninRepository for recording login history.
func (h *Handler) SetSigninRepo(repo repository.SigninRepository, idGen id.Generator) {
	h.signinRepo = repo
	h.idGen = idGen
}

// SetCaptcha attaches a CaptchaService. When set, signin-flow verifies
// the captcha token on the password step (same as original Misskey).
func (h *Handler) SetCaptcha(svc *captcha.Service) {
	h.captchaSvc = svc
}

// SetWebAuthn attaches optional WebAuthn dependencies to enable 2FA login
// via security keys. Without it the handler still supports TOTP and backup
// codes (and falls back to single-factor when 2FA is disabled).
func (h *Handler) SetWebAuthn(svc *twofactor.WebAuthnService, repo repository.UserSecurityKeyRepository) {
	h.webauthnSvc = svc
	h.securityKeyRepo = repo
}

// Signin handles POST /api/signin.
// Misskey フロントエンドのログインフロー:
// 1. username のみ → { finished: false, next: "password" or "captcha" }
// 2. username + password → { finished: true, id: "...", i: "token" }
func (h *Handler) Signin(c echo.Context) error {
	var req struct {
		Username string  `json:"username"`
		Password *string `json:"password"`
	}
	if err := c.Bind(&req); err != nil || req.Username == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error": map[string]any{
				"id": "6cc579cc-885d-43d8-95c2-b8c7fc963280",
			},
		})
	}

	// ユーザー検索
	user, err := h.userRepo.FindByUsernameLower(req.Username, nil)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]any{
			"error": map[string]any{
				"id": "6cc579cc-885d-43d8-95c2-b8c7fc963280",
			},
		})
	}

	if user.IsSuspended {
		return c.JSON(http.StatusForbidden, map[string]any{
			"error": map[string]any{
				"id": "e03a5f46-d309-4865-9b69-56282d94e1eb",
			},
		})
	}

	// Step 1: パスワードなしの場合、次のステップを返す
	if req.Password == nil {
		return c.JSON(http.StatusOK, map[string]any{
			"finished": false,
			"next":     "password",
		})
	}

	// Step 2: パスワード検証
	profile, err := h.userRepo.FindProfileByUserID(user.ID)
	if err != nil || profile.Password == nil {
		return c.JSON(http.StatusForbidden, map[string]any{
			"error": map[string]any{
				"id": "932c904e-9460-45b7-9ce6-7ed33be7eb2c",
			},
		})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*profile.Password), []byte(*req.Password)); err != nil {
		return c.JSON(http.StatusForbidden, map[string]any{
			"error": map[string]any{
				"id": "932c904e-9460-45b7-9ce6-7ed33be7eb2c",
			},
		})
	}

	// 認証成功
	return h.ok(c, user)
}

// SigninFlow handles POST /api/signin-flow.
// TS本家のmulti-step ログインフロー (Misskey 2026.x):
//
//	Step 1: { username }                           → { next: "password" }
//	Step 2: { username, password }                 → 2FA 無し: { finished, id, i }
//	                                               → TOTP / WebAuthn 有り: { next: "totp" / "captcha-keys" }
//	Step 3 (TOTP):  { username, password, token }  → { finished, id, i }
//	Step 3 (BU):    { username, password, token }  → backup code 検証
//	Step 3 (Key):   { username, password, credential, ...session } → WebAuthn assertion 検証
func (h *Handler) SigninFlow(c echo.Context) error {
	var req struct {
		Username string  `json:"username"`
		Password *string `json:"password"`
		Token    *string `json:"token"`
		// SessionID + Credential はキー認証 step 3 で使う。
		SessionID  *string         `json:"sessionId"`
		Credential json.RawMessage `json:"credential"`
		// CAPTCHA tokens — フロントエンドは有効な provider の token だけ送る。
		HcaptchaResponse    string `json:"hcaptcha-response"`
		RecaptchaResponse   string `json:"g-recaptcha-response"`
		TurnstileResponse   string `json:"turnstile-response"`
		McaptchaResponse    string `json:"m-captcha-response"`
		TestcaptchaResponse string `json:"testcaptcha-response"`
	}
	if err := c.Bind(&req); err != nil || req.Username == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error": map[string]any{
				"id": "6cc579cc-885d-43d8-95c2-b8c7fc963280",
			},
		})
	}

	// ユーザー検索 (小文字で検索)
	user, err := h.userRepo.FindByUsernameLower(req.Username, nil)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]any{
			"error": map[string]any{
				"id": "6cc579cc-885d-43d8-95c2-b8c7fc963280",
			},
		})
	}

	if user.IsSuspended {
		return c.JSON(http.StatusForbidden, map[string]any{
			"error": map[string]any{
				"id": "e03a5f46-d309-4865-9b69-56282d94e1eb",
			},
		})
	}

	// Step 1: パスワード未提供 → 次のステップを指示
	if req.Password == nil {
		return c.JSON(http.StatusOK, map[string]any{
			"finished": false,
			"next":     "password",
		})
	}

	// Step 2: パスワード検証
	profile, err := h.userRepo.FindProfileByUserID(user.ID)
	if err != nil || profile.Password == nil {
		return c.JSON(http.StatusForbidden, map[string]any{
			"error": map[string]any{
				"id": "932c904e-9460-45b7-9ce6-7ed33be7eb2c",
			},
		})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*profile.Password), []byte(*req.Password)); err != nil {
		return c.JSON(http.StatusForbidden, map[string]any{
			"error": map[string]any{
				"id": "932c904e-9460-45b7-9ce6-7ed33be7eb2c",
			},
		})
	}

	// CAPTCHA 検証 (password step 完了後、2FA 無しの場合のみ)。
	// 本家 Misskey と同じく 2FA 有効なユーザーはキーデバイスが人間性を担保する
	// ため CAPTCHA をスキップする。
	if !profile.TwoFactorEnabled && h.captchaSvc != nil {
		tokens := captcha.CaptchaTokens{
			Hcaptcha:    req.HcaptchaResponse,
			Recaptcha:   req.RecaptchaResponse,
			Turnstile:   req.TurnstileResponse,
			Mcaptcha:    req.McaptchaResponse,
			Testcaptcha: req.TestcaptchaResponse,
		}
		if err := h.captchaSvc.Verify(c.Request().Context(), tokens); err != nil {
			return c.JSON(http.StatusForbidden, errBody("e03a5f46-d309-4865-9b69-56282d94e1eb"))
		}
	}

	// 2FA 経路: TwoFactorEnabled が立っていたら 2 要素を要求する。
	if profile.TwoFactorEnabled {
		// security key を持っていれば WebAuthn step を最初に提示する (assertion challenge)。
		hasKeys := false
		var keys []*model.UserSecurityKey
		if h.webauthnSvc != nil && h.securityKeyRepo != nil {
			ks, err := h.securityKeyRepo.ListByUser(user.ID)
			if err == nil && len(ks) > 0 {
				hasKeys = true
				keys = ks
			}
		}

		// Step 3 (Key): credential が来ていれば WebAuthn assertion を検証する。
		if hasKeys && len(req.Credential) > 0 && req.SessionID != nil {
			httpReq, werr := wrapWebAuthnRequest(c.Request(), req.Credential)
			if werr != nil {
				return c.JSON(http.StatusBadRequest, errBody("ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
			}
			cred, werr := h.webauthnSvc.FinishLogin(c.Request().Context(), user, keys, *req.SessionID, httpReq)
			if werr != nil {
				return c.JSON(http.StatusForbidden, errBody("932c904e-9460-45b7-9ce6-7ed33be7eb2c"))
			}
			// counter 更新
			if h.securityKeyRepo != nil {
				_ = h.securityKeyRepo.UpdateCounter(encodeCredID(cred.ID), int64(cred.Authenticator.SignCount))
			}
			return h.ok(c, user)
		}

		// Step 3 (TOTP / Backup): token フィールドで 2FA を検証する。
		if req.Token != nil && *req.Token != "" {
			// まず TOTP を試す。失敗したらバックアップコードにフォールバック。
			if profile.TwoFactorSecret != nil && twofactor.Validate(*req.Token, *profile.TwoFactorSecret) {
				return h.ok(c, user)
			}
			if remaining, berr := twofactor.ConsumeBackupCode([]string(profile.TwoFactorBackupSecret), *req.Token); berr == nil {
				_ = h.userRepo.UpdateProfile(user.ID, map[string]any{
					"twoFactorBackupSecret": pq.StringArray(remaining),
				})
				return h.ok(c, user)
			}
			return c.JSON(http.StatusForbidden, errBody("932c904e-9460-45b7-9ce6-7ed33be7eb2c"))
		}

		// 2FA が必要だが何も渡されていない → 次のステップを指示する。
		next := "totp"
		respBody := map[string]any{
			"finished": false,
			"next":     next,
		}
		// security key 保有なら assertion challenge を発行する。
		if hasKeys && h.webauthnSvc != nil {
			assertion, sid, werr := h.webauthnSvc.BeginLogin(c.Request().Context(), user, keys)
			if werr == nil {
				respBody["next"] = "captcha-keys"
				respBody["sessionId"] = sid
				respBody["assertion"] = assertion
			}
		}
		return c.JSON(http.StatusOK, respBody)
	}

	// 認証成功 — トークン返却
	return h.ok(c, user)
}

// ok returns the standard "logged in" response. 認証経路 (TOTP / WebAuthn /
// pwd-only) すべてで同じ shape を返したいのでヘルパに切り出している。
func (h *Handler) ok(c echo.Context, user *model.User) error {
	if h.ipLoggingOn && h.ipLogger != nil {
		go h.ipLogger.Upsert(user.ID, c.RealIP())
	}
	// signinレコードを非同期で記録
	if h.signinRepo != nil && h.idGen != nil {
		go h.recordSignin(user.ID, c.RealIP(), c.Request().Header)
	}
	token := ""
	if user.Token != nil {
		token = *user.Token
	}
	return c.JSON(http.StatusOK, map[string]any{
		"finished": true,
		"id":       user.ID,
		"i":        token,
	})
}

// sanitizeHeaders removes sensitive headers before persisting.
func sanitizeHeaders(h http.Header) http.Header {
	safe := h.Clone()
	safe.Del("Authorization")
	safe.Del("Cookie")
	safe.Del("Set-Cookie")
	return safe
}

// recordSignin persists a signin record asynchronously.
func (h *Handler) recordSignin(userID, ip string, headers http.Header) {
	hdrs, err := json.Marshal(sanitizeHeaders(headers))
	if err != nil {
		hdrs = []byte("{}")
	}
	now := time.Now()
	s := &model.Signin{
		ID:      h.idGen.Generate(now),
		UserID:  userID,
		IP:      ip,
		Headers: datatypes.JSON(hdrs),
		Success: true,
	}
	if err := h.signinRepo.Create(s); err != nil {
		slog.Warn("failed to record signin", "userId", userID, "err", err)
	}
}

func errBody(id string) map[string]any {
	return map[string]any{
		"error": map[string]any{"id": id},
	}
}

// wrapWebAuthnRequest builds a fresh *http.Request whose body is the
// browser-supplied attestation/assertion JSON. go-webauthn parses the body
// directly off the request, so we cannot pass through the original Echo
// request (its body has already been consumed by Bind()).
func wrapWebAuthnRequest(orig *http.Request, body json.RawMessage) (*http.Request, error) {
	req, err := http.NewRequestWithContext(orig.Context(), http.MethodPost, orig.URL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header = orig.Header.Clone()
	req.Header.Set("Content-Type", "application/json")
	req.Body = io.NopCloser(bytes.NewReader(body))
	return req, nil
}

// encodeCredID returns the storage key (base64url) for a webauthn credential
// id. Centralizing the encoding here avoids drift between handler_2fa.go's
// CredentialToModel and signin's counter update path.
func encodeCredID(raw []byte) string {
	return base64.RawURLEncoding.EncodeToString(raw)
}
