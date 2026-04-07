package activitypub

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateAndParseKeypair(t *testing.T) {
	priv, pub, err := GenerateRSAKeypair()
	require.NoError(t, err)
	assert.Contains(t, priv, "RSA PRIVATE KEY")
	assert.Contains(t, pub, "PUBLIC KEY")

	rsaPriv, err := ParseRSAPrivateKey(priv)
	require.NoError(t, err)
	assert.NotNil(t, rsaPriv)

	rsaPub, err := ParseRSAPublicKey(pub)
	require.NoError(t, err)
	assert.NotNil(t, rsaPub)
}

func TestParseRSAPrivateKey_Invalid(t *testing.T) {
	_, err := ParseRSAPrivateKey("not pem")
	assert.Error(t, err)
}

func TestParseRSAPrivateKey_BadContents(t *testing.T) {
	pem := "-----BEGIN RSA PRIVATE KEY-----\nbm90YWtleQ==\n-----END RSA PRIVATE KEY-----\n"
	_, err := ParseRSAPrivateKey(pem)
	assert.Error(t, err)
}

func TestParseRSAPublicKey_Invalid(t *testing.T) {
	_, err := ParseRSAPublicKey("not pem")
	assert.Error(t, err)
}

func TestParseRSAPublicKey_BadContents(t *testing.T) {
	pem := "-----BEGIN PUBLIC KEY-----\nbm90YWtleQ==\n-----END PUBLIC KEY-----\n"
	_, err := ParseRSAPublicKey(pem)
	assert.Error(t, err)
}

func TestParseRSAPublicKey_NotRSA(t *testing.T) {
	// Ed25519 鍵を生成して PEM にエンコードし、ParseRSAPublicKey の
	// 「not RSA」分岐を踏ませる。
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	derBytes, err := x509.MarshalPKIXPublicKey(pub)
	require.NoError(t, err)
	pemStr := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: derBytes}))

	_, err = ParseRSAPublicKey(pemStr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not an RSA")
}

// errReader is an io.Reader that always errors.
type errReader struct{}

func (errReader) Read(_ []byte) (int, error) { return 0, assertErr }

var assertErr = newSentinelErr("rand fail")

type sentinelErr struct{ msg string }

func (e *sentinelErr) Error() string { return e.msg }

func newSentinelErr(s string) error { return &sentinelErr{msg: s} }

func TestGenerateRSAKeypair_RandError(t *testing.T) {
	restore := SetRandReaderForTest(errReader{})
	defer restore()

	_, _, err := GenerateRSAKeypair()
	require.Error(t, err)
}
