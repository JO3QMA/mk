package mediaproxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSignURL(t *testing.T) {
	secret := []byte("test-secret-key")
	url := "https://remote.example/avatar.png"

	sig := SignURL(secret, url)
	assert.NotEmpty(t, sig)
	assert.Len(t, sig, 64) // SHA-256 = 32 bytes = 64 hex chars
}

func TestVerifyHMAC_Valid(t *testing.T) {
	secret := []byte("test-secret-key")
	url := "https://remote.example/avatar.png"

	sig := SignURL(secret, url)
	assert.True(t, VerifyHMAC(secret, url, sig))
}

func TestVerifyHMAC_WrongSignature(t *testing.T) {
	secret := []byte("test-secret-key")
	url := "https://remote.example/avatar.png"

	assert.False(t, VerifyHMAC(secret, url, "deadbeef"))
}

func TestVerifyHMAC_WrongSecret(t *testing.T) {
	secret := []byte("test-secret-key")
	wrongSecret := []byte("wrong-secret")
	url := "https://remote.example/avatar.png"

	sig := SignURL(secret, url)
	assert.False(t, VerifyHMAC(wrongSecret, url, sig))
}

func TestVerifyHMAC_WrongURL(t *testing.T) {
	secret := []byte("test-secret-key")
	url := "https://remote.example/avatar.png"

	sig := SignURL(secret, url)
	assert.False(t, VerifyHMAC(secret, "https://evil.example/malware.exe", sig))
}

func TestVerifyHMAC_EmptyInputs(t *testing.T) {
	secret := []byte("test-secret-key")

	// 空URLでもsign/verifyは成功する（URLバリデーションは上位層の責務）
	sig := SignURL(secret, "")
	assert.True(t, VerifyHMAC(secret, "", sig))

	// 空シグネチャは不一致
	assert.False(t, VerifyHMAC(secret, "https://example.com", ""))
}

func TestSignURL_Deterministic(t *testing.T) {
	secret := []byte("test-secret-key")
	url := "https://remote.example/avatar.png"

	sig1 := SignURL(secret, url)
	sig2 := SignURL(secret, url)
	assert.Equal(t, sig1, sig2)
}

func TestSignURL_DifferentURLsProduceDifferentSignatures(t *testing.T) {
	secret := []byte("test-secret-key")

	sig1 := SignURL(secret, "https://example.com/a.png")
	sig2 := SignURL(secret, "https://example.com/b.png")
	assert.NotEqual(t, sig1, sig2)
}
