package captcha

import "context"

// testcaptchaMagic is the exact string the frontend sends when testcaptcha
// is enabled. Must match the original Misskey implementation.
const testcaptchaMagic = "testcaptcha-passed"

type testcaptchaVerifier struct{}

// NewTestcaptcha returns a Verifier for the development/E2E testcaptcha.
// 本番環境では絶対に enableTestcaptcha を有効にしてはならない。
func NewTestcaptcha() Verifier {
	return &testcaptchaVerifier{}
}

func (v *testcaptchaVerifier) Verify(_ context.Context, token string) error {
	if token == "" {
		return ErrNoResponse
	}
	if token != testcaptchaMagic {
		return ErrVerificationFail
	}
	return nil
}
