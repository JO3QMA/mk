package misc

import (
	"crypto/rand"
	"encoding/hex"
)

// SecureRandomHex returns a hex-encoded random string of n characters.
func SecureRandomHex(n int) string {
	b := make([]byte, (n+1)/2)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)[:n]
}
