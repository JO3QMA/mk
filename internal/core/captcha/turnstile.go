package captcha

import "net/http"

const turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// NewTurnstile returns a Verifier for Cloudflare Turnstile.
func NewTurnstile(secret string) Verifier {
	return &siteVerifier{verifyURL: turnstileVerifyURL, secret: secret}
}

// NewTurnstileWithClient allows injecting an *http.Client for tests.
func NewTurnstileWithClient(secret string, client *http.Client) Verifier {
	return &siteVerifier{verifyURL: turnstileVerifyURL, secret: secret, client: client}
}

// NewTurnstileWithURL allows overriding both URL and client (for httptest).
func NewTurnstileWithURL(secret, url string, client *http.Client) Verifier {
	return &siteVerifier{verifyURL: url, secret: secret, client: client}
}
