// Package twofactor provides TOTP-based two-factor authentication.
package twofactor

import (
	"github.com/pquerna/otp/totp"
)

// GenerateSecret creates a new TOTP secret for a user.
// Returns the secret key and the otpauth:// URI for QR code generation.
func GenerateSecret(issuer, accountName string) (secret string, uri string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: accountName,
	})
	if err != nil {
		return "", "", err
	}
	return key.Secret(), key.URL(), nil
}

// Validate checks if a TOTP code is valid for the given secret.
func Validate(code, secret string) bool {
	return totp.Validate(code, secret)
}
