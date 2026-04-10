package twofactor

import (
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSecret(t *testing.T) {
	secret, uri, err := GenerateSecret("Misskey", "testuser")
	require.NoError(t, err)
	assert.NotEmpty(t, secret)
	assert.Contains(t, uri, "otpauth://totp/")
	assert.Contains(t, uri, "Misskey")
}

func TestValidate_Success(t *testing.T) {
	secret, _, err := GenerateSecret("Misskey", "testuser")
	require.NoError(t, err)
	// 正しいコードを生成して検証
	code, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err)
	assert.True(t, Validate(code, secret))
}

func TestValidate_WrongCode(t *testing.T) {
	secret, _, err := GenerateSecret("Misskey", "testuser")
	require.NoError(t, err)
	assert.False(t, Validate("000000", secret))
}

func TestValidate_EmptySecret(t *testing.T) {
	assert.False(t, Validate("123456", ""))
}

func TestGenerateSecret_EmptyIssuer(t *testing.T) {
	_, _, err := GenerateSecret("", "")
	// 空issuerはエラーになる → エラーパスをカバー
	assert.Error(t, err)
}
