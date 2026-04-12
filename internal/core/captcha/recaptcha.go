package captcha

import "net/http"

const recaptchaVerifyURL = "https://www.recaptcha.net/recaptcha/api/siteverify"

// NewRecaptcha returns a Verifier for Google reCAPTCHA.
func NewRecaptcha(secret string) Verifier {
	return &siteVerifier{verifyURL: recaptchaVerifyURL, secret: secret}
}

// NewRecaptchaWithClient allows injecting an *http.Client for tests.
func NewRecaptchaWithClient(secret string, client *http.Client) Verifier {
	return &siteVerifier{verifyURL: recaptchaVerifyURL, secret: secret, client: client}
}
