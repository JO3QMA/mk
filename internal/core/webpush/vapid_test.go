package webpush

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateVAPIDKeys(t *testing.T) {
	pub, priv, err := GenerateVAPIDKeys()
	require.NoError(t, err)

	pubBytes, err := base64.RawURLEncoding.DecodeString(pub)
	require.NoError(t, err)
	// P-256 uncompressed point: 0x04 || X(32) || Y(32) = 65 bytes
	assert.Len(t, pubBytes, 65)
	assert.Equal(t, byte(0x04), pubBytes[0])

	privBytes, err := base64.RawURLEncoding.DecodeString(priv)
	require.NoError(t, err)
	assert.Len(t, privBytes, 32)
}

// 2 回生成して必ず別の鍵が返ること (= 乱数源が固定でない)。
func TestGenerateVAPIDKeys_RandomEachCall(t *testing.T) {
	pub1, priv1, err := GenerateVAPIDKeys()
	require.NoError(t, err)
	pub2, priv2, err := GenerateVAPIDKeys()
	require.NoError(t, err)
	assert.NotEqual(t, pub1, pub2)
	assert.NotEqual(t, priv1, priv2)
}
