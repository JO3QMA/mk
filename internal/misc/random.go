package misc

import (
	"crypto/rand"
	"encoding/hex"
	"io"
)

// randReader is the source of cryptographic randomness.
// テスト時に差し替え可能にするためパッケージ変数として公開
var randReader io.Reader = rand.Reader

// SecureRandomHex returns a hex-encoded random string of n characters.
func SecureRandomHex(n int) string {
	b := make([]byte, (n+1)/2)
	if _, err := io.ReadFull(randReader, b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)[:n]
}
