package activitypub

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"time"
)

// randReader is the entropy source used by GenerateRSAKeypair. テストで差し替え
// 可能なように package-level の var にしている。
var randReader io.Reader = rand.Reader

// nowFunc returns the current time. テストで差し替え可能。
var nowFunc = time.Now

// GenerateRSAKeypair returns a fresh 2048-bit RSA keypair encoded as PEM
// strings (private + public). 失敗時はエラーを返す。MarshalPKIXPublicKey は
// RSAキーに対しては常に成功するため、その後のエラーチェックは省略している。
func GenerateRSAKeypair() (privatePEM string, publicPEM string, err error) {
	priv, err := rsa.GenerateKey(randReader, 2048)
	if err != nil {
		return "", "", err
	}
	privBytes := x509.MarshalPKCS1PrivateKey(priv)
	privBlock := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privBytes}

	pubBytes, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	pubBlock := &pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}

	return string(pem.EncodeToMemory(privBlock)), string(pem.EncodeToMemory(pubBlock)), nil
}

// ParseRSAPrivateKey decodes a PEM-encoded RSA private key.
func ParseRSAPrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("invalid PEM block")
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

// ParseRSAPublicKey decodes a PEM-encoded RSA public key.
func ParseRSAPublicKey(pemStr string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("invalid PEM block")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("not an RSA public key")
	}
	return rsaPub, nil
}
