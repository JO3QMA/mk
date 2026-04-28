package webpush

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// GenerateVAPIDKeys returns a fresh VAPID key pair encoded as base64
// url-safe strings (no padding), matching the wire format expected by
// browser push subscriptions (RFC 8291) and the upstream Misskey TS
// `web-push.generateVAPIDKeys()` output.
//
// Public key is the uncompressed P-256 point (65 bytes, leading 0x04)
// base64url-encoded. Private key is the 32-byte raw scalar.
//
// crypto/ecdh の P256 を使う。crypto/ecdsa + elliptic.Marshal は Go 1.22 で
// deprecated になっているが、ecdh.PublicKey.Bytes() / PrivateKey.Bytes()
// が同じ wire format (65 byte uncompressed point / 32 byte scalar) を
// 返してくれるのでそのまま web push 用途に使える。
func GenerateVAPIDKeys() (publicKey, privateKey string, err error) {
	priv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("webpush: generate ECDH P-256 key: %w", err)
	}
	enc := base64.RawURLEncoding
	return enc.EncodeToString(priv.PublicKey().Bytes()), enc.EncodeToString(priv.Bytes()), nil
}
