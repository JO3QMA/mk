package captcha

import "net/http"

const hcaptchaVerifyURL = "https://hcaptcha.com/siteverify"

// NewHcaptcha returns a Verifier for hCaptcha.
func NewHcaptcha(secret string) Verifier {
	return &siteVerifier{verifyURL: hcaptchaVerifyURL, secret: secret}
}

// NewHcaptchaWithClient allows injecting an *http.Client for tests.
func NewHcaptchaWithClient(secret string, client *http.Client) Verifier {
	return &siteVerifier{verifyURL: hcaptchaVerifyURL, secret: secret, client: client}
}
