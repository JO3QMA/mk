// Package captcha provides pluggable CAPTCHA verification for signup and
// signin flows. Each provider (hCaptcha, reCAPTCHA, Turnstile, mCaptcha,
// testcaptcha) implements the Verifier interface. Service reads the active
// Meta config and delegates to the first enabled provider.
package captcha

import (
	"context"
	"errors"

	"github.com/shiroha-a/mk/internal/model"
)

// Errors returned by captcha verification.
var (
	ErrNoResponse       = errors.New("captcha: no response provided")
	ErrVerificationFail = errors.New("captcha: verification failed")
	ErrRequestFailed    = errors.New("captcha: request to provider failed")
)

// Verifier validates a captcha response token against its provider.
type Verifier interface {
	Verify(ctx context.Context, token string) error
}

// CaptchaTokens bundles all possible captcha response tokens sent by the
// frontend. Only the one matching the enabled provider is inspected.
type CaptchaTokens struct {
	Hcaptcha    string
	Recaptcha   string
	Turnstile   string
	Mcaptcha    string
	Testcaptcha string
}

// Service selects the active captcha provider from meta config and verifies
// the corresponding token. If no provider is enabled, verification succeeds
// unconditionally (captcha is optional).
type Service struct {
	hcaptcha  Verifier
	recaptcha Verifier
	turnstile Verifier
	mcaptcha  Verifier
	testcap   Verifier
}

// NewService builds a Service from the given meta. Only enabled providers
// are instantiated; callers pass nil HTTPClient to use http.DefaultClient.
func NewService(meta *model.Meta) *Service {
	s := &Service{}

	if meta.EnableHcaptcha && meta.HcaptchaSecretKey != nil {
		s.hcaptcha = NewHcaptcha(*meta.HcaptchaSecretKey)
	}
	if meta.EnableRecaptcha && meta.RecaptchaSecretKey != nil {
		s.recaptcha = NewRecaptcha(*meta.RecaptchaSecretKey)
	}
	if meta.EnableTurnstile && meta.TurnstileSecretKey != nil {
		s.turnstile = NewTurnstile(*meta.TurnstileSecretKey)
	}
	if meta.EnableMcaptcha && meta.McaptchaSecretKey != nil && meta.McaptchaInstanceURL != nil {
		siteKey := ""
		if meta.McaptchaSiteKey != nil {
			siteKey = *meta.McaptchaSiteKey
		}
		s.mcaptcha = NewMcaptcha(*meta.McaptchaInstanceURL, siteKey, *meta.McaptchaSecretKey)
	}
	if meta.EnableTestcaptcha {
		s.testcap = NewTestcaptcha()
	}

	return s
}

// Verify checks the token matching the first enabled provider. Returns nil
// if no provider is enabled (captcha disabled).
func (s *Service) Verify(ctx context.Context, tokens CaptchaTokens) error {
	switch {
	case s.hcaptcha != nil:
		return s.hcaptcha.Verify(ctx, tokens.Hcaptcha)
	case s.recaptcha != nil:
		return s.recaptcha.Verify(ctx, tokens.Recaptcha)
	case s.turnstile != nil:
		return s.turnstile.Verify(ctx, tokens.Turnstile)
	case s.mcaptcha != nil:
		return s.mcaptcha.Verify(ctx, tokens.Mcaptcha)
	case s.testcap != nil:
		return s.testcap.Verify(ctx, tokens.Testcaptcha)
	default:
		return nil
	}
}
