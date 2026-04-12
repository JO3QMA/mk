// Package email provides pluggable email validation for signup and
// update-email flows. Validators (format, MX, verifymail, truemail,
// bannedDomains) are composed by Service based on the active Meta config.
package email

import (
	"context"
	"errors"
	"strings"

	"github.com/shiroha-a/mk/internal/model"
)

// Validation failure reasons — match the original Misskey's
// EmailService.validateEmailForAccount return values.
var (
	ErrFormat     = errors.New("email: invalid format")
	ErrMX         = errors.New("email: no MX record")
	ErrDisposable = errors.New("email: disposable address")
	ErrSMTP       = errors.New("email: smtp validation failed")
	ErrBanned     = errors.New("email: domain is banned")
	ErrNetwork    = errors.New("email: network error")
	ErrBlacklist  = errors.New("email: blacklisted")
)

// Service validates an email address against all enabled checks.
type Service struct {
	bannedDomains []string
	activeCheck   bool
	verifymail    *verifymailClient
	truemail      *truemailClient
}

// NewService builds a Service from the given meta config.
func NewService(meta *model.Meta) *Service {
	s := &Service{
		bannedDomains: meta.BannedEmailDomains,
	}

	if meta.EnableVerifymailAPI && meta.VerifymailAuthKey != nil && *meta.VerifymailAuthKey != "" {
		s.verifymail = newVerifymailClient(*meta.VerifymailAuthKey)
	} else if meta.EnableTruemailAPI && meta.TruemailInstance != nil && meta.TruemailAuthKey != nil {
		s.truemail = newTruemailClient(*meta.TruemailInstance, *meta.TruemailAuthKey)
	} else if meta.EnableActiveEmailValidation {
		s.activeCheck = true
	}

	return s
}

// Validate checks the email address against all enabled validations:
// 1. Format (always)
// 2. Banned domains (always, if list non-empty)
// 3. External service or MX check (if configured)
func (s *Service) Validate(ctx context.Context, email string) error {
	if !ValidateFormat(email) {
		return ErrFormat
	}

	domain := domainOf(email)
	if IsBannedDomain(domain, s.bannedDomains) {
		return ErrBanned
	}

	switch {
	case s.verifymail != nil:
		return s.verifymail.verify(ctx, email)
	case s.truemail != nil:
		return s.truemail.verify(ctx, email)
	case s.activeCheck:
		return CheckMX(domain)
	}

	return nil
}

// domainOf extracts the domain part from an email address.
// Assumes format validation has already passed.
func domainOf(email string) string {
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return ""
	}
	return strings.ToLower(email[at+1:])
}
