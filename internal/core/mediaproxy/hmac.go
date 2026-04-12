package mediaproxy

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// SignURL produces a hex-encoded HMAC-SHA256 signature of the given URL.
func SignURL(secret []byte, rawURL string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(rawURL))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyHMAC checks that sig matches the HMAC-SHA256 of rawURL.
// Uses constant-time comparison to prevent timing attacks.
func VerifyHMAC(secret []byte, rawURL, sig string) bool {
	expected := SignURL(secret, rawURL)
	return hmac.Equal([]byte(expected), []byte(sig))
}
