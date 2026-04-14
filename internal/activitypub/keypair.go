package activitypub

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"time"
)

// randReader is the entropy source used by GenerateRSAKeypair. テストで差し替え
// 可能なように package-level の var にしている。
var randReader io.Reader = rand.Reader

// nowFunc returns the current time. テストで差し替え可能。
var nowFunc = time.Now

// KeyType discriminates between supported HTTP Signature key algorithms.
// RSA は Mastodon/Misskey の事実上の標準、Ed25519 は新世代 Fediverse 実装で
// 採用されつつある軽量・高速な署名方式。
type KeyType int

const (
	// KeyTypeRSA は RSA-SHA256 / RSA-SHA512 / hs2019(RSA) で使われる。
	KeyTypeRSA KeyType = iota
	// KeyTypeEd25519 は ed25519 / hs2019(Ed25519) で使われる。
	KeyTypeEd25519
)

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

// GenerateEd25519Keypair returns a fresh Ed25519 keypair encoded as PEM
// strings (private + public). 鍵生成は io エラーの時だけ失敗するため、
// MarshalPKCS8PrivateKey / MarshalPKIXPublicKey 側のエラーチェックは省略。
func GenerateEd25519Keypair() (privatePEM string, publicPEM string, err error) {
	pub, priv, err := ed25519.GenerateKey(randReader)
	if err != nil {
		return "", "", err
	}
	privBytes, _ := x509.MarshalPKCS8PrivateKey(priv)
	privBlock := &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}

	pubBytes, _ := x509.MarshalPKIXPublicKey(pub)
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

// ParseEd25519PrivateKey decodes a PEM-encoded Ed25519 private key (PKCS8).
func ParseEd25519PrivateKey(pemStr string) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("invalid PEM block")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	priv, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("not an Ed25519 private key")
	}
	return priv, nil
}

// ParseRSAPublicKey decodes a PEM-encoded RSA public key.
// 既存呼び出し側の互換のため残す。新規コードは ParsePublicKey を使うこと。
func ParseRSAPublicKey(pemStr string) (*rsa.PublicKey, error) {
	pub, kt, err := ParsePublicKey(pemStr)
	if err != nil {
		return nil, err
	}
	if kt != KeyTypeRSA {
		return nil, errors.New("not an RSA public key")
	}
	return pub.(*rsa.PublicKey), nil
}

// ParsePublicKey decodes a PEM-encoded public key and reports its type.
// RSA / Ed25519 を識別できる。それ以外は "unsupported public key type" で
// 拒否する (Misskey フェデレーションでは ECDSA 等は事実上使われていない)。
func ParsePublicKey(pemStr string) (crypto.PublicKey, KeyType, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, 0, errors.New("invalid PEM block")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, 0, err
	}
	switch k := pub.(type) {
	case *rsa.PublicKey:
		return k, KeyTypeRSA, nil
	case ed25519.PublicKey:
		return k, KeyTypeEd25519, nil
	default:
		return nil, 0, fmt.Errorf("unsupported public key type: %T", pub)
	}
}
