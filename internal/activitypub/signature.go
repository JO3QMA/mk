package activitypub

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// PrivateKey bundles a parsed RSA private key with its public keyId URI.
// keyId は他サーバーが公開鍵を取得するための URI (例: https://example.com/users/u1#main-key)。
type PrivateKey struct {
	KeyID      string
	PrivatePEM string
	parsed     *rsa.PrivateKey
}

// NewPrivateKey wraps a PEM string into a PrivateKey, parsing it once.
func NewPrivateKey(keyID, privatePEM string) (*PrivateKey, error) {
	parsed, err := ParseRSAPrivateKey(privatePEM)
	if err != nil {
		return nil, err
	}
	return &PrivateKey{KeyID: keyID, PrivatePEM: privatePEM, parsed: parsed}, nil
}

// SHA256Digest returns the "SHA-256=<base64>" digest header value for body.
func SHA256Digest(body []byte) string {
	sum := sha256.Sum256(body)
	return "SHA-256=" + base64.StdEncoding.EncodeToString(sum[:])
}

// SignRequest mutates req to include the Date, Host, Digest (when bodyDigest!="")
// and Signature headers required by the Cavage HTTP Signatures draft v12.
//
// includeHeaders はヘッダ名の小文字リスト。"(request-target)" を含めると
// "(request-target): <method> <path>" が署名対象に追加される。
func SignRequest(req *http.Request, key *PrivateKey, bodyDigest string, includeHeaders []string) error {
	if key == nil || key.parsed == nil {
		return errors.New("private key required")
	}

	if req.Header.Get("Date") == "" {
		req.Header.Set("Date", nowFunc().UTC().Format(http.TimeFormat))
	}
	if req.Header.Get("Host") == "" {
		req.Header.Set("Host", req.URL.Host)
	}
	if bodyDigest != "" {
		req.Header.Set("Digest", bodyDigest)
	}

	signingString, err := buildSigningString(req, includeHeaders)
	if err != nil {
		return err
	}

	hashed := sha256.Sum256([]byte(signingString))
	// rsa.SignPKCS1v15 は PKCS1v15 では rand を使わないため失敗しない (RSA-PSS の場合は使う)。
	sig, _ := rsa.SignPKCS1v15(randReader, key.parsed, crypto.SHA256, hashed[:])

	header := fmt.Sprintf(
		`keyId="%s",algorithm="rsa-sha256",headers="%s",signature="%s"`,
		key.KeyID,
		strings.Join(includeHeaders, " "),
		base64.StdEncoding.EncodeToString(sig),
	)
	req.Header.Set("Signature", header)
	// Hostヘッダはnet/httpがリクエストラインから自動付与するため取り除く。
	req.Header.Del("Host")
	return nil
}

// buildSigningString concatenates the canonical signing string for the
// requested headers.
func buildSigningString(req *http.Request, includeHeaders []string) (string, error) {
	parts := make([]string, 0, len(includeHeaders))
	for _, raw := range includeHeaders {
		key := strings.ToLower(raw)
		if key == "(request-target)" {
			parts = append(parts, fmt.Sprintf("(request-target): %s %s", strings.ToLower(req.Method), req.URL.RequestURI()))
			continue
		}
		var value string
		if key == "host" {
			// SignRequest が事前に Host を埋めているが、Verify 経路では
			// 受信側 net/http が r.Host にしかセットしないことがある。
			value = req.Header.Get("Host")
			if value == "" {
				value = req.Host
			}
		} else {
			value = req.Header.Get(key)
		}
		if value == "" {
			return "", fmt.Errorf("missing required header %q for signature", key)
		}
		parts = append(parts, fmt.Sprintf("%s: %s", key, value))
	}
	return strings.Join(parts, "\n"), nil
}

// ParsedSignature is the structured form of a Signature header.
type ParsedSignature struct {
	KeyID     string
	Algorithm string
	Headers   []string
	Signature string
}

var sigKVPattern = regexp.MustCompile(`(\w+)="([^"]+)"`)

// ParseSignatureHeader parses an HTTP Signature header into its components.
func ParseSignatureHeader(header string) (*ParsedSignature, error) {
	if header == "" {
		return nil, errors.New("missing signature header")
	}
	out := &ParsedSignature{}
	for _, m := range sigKVPattern.FindAllStringSubmatch(header, -1) {
		switch m[1] {
		case "keyId":
			out.KeyID = m[2]
		case "algorithm":
			out.Algorithm = m[2]
		case "headers":
			out.Headers = strings.Fields(m[2])
		case "signature":
			out.Signature = m[2]
		}
	}
	if out.KeyID == "" || out.Signature == "" {
		return nil, errors.New("invalid signature header: keyId and signature are required")
	}
	if len(out.Headers) == 0 {
		// RFC default = ["date"]
		out.Headers = []string{"date"}
	}
	return out, nil
}

// VerifyRequest verifies an incoming HTTP request signature against the
// supplied PEM public key. requestURI overrides req.URL.RequestURI() to allow
// callers to feed the original raw path (echo's c.Request().URL is normalized
// in some cases).
func VerifyRequest(req *http.Request, publicKeyPEM string) error {
	parsed, err := ParseSignatureHeader(req.Header.Get("Signature"))
	if err != nil {
		return err
	}
	if parsed.Algorithm != "" && !strings.EqualFold(parsed.Algorithm, "rsa-sha256") {
		return fmt.Errorf("unsupported algorithm %q", parsed.Algorithm)
	}
	pub, err := ParseRSAPublicKey(publicKeyPEM)
	if err != nil {
		return err
	}
	signingString, err := buildSigningString(req, parsed.Headers)
	if err != nil {
		return err
	}
	sig, err := base64.StdEncoding.DecodeString(parsed.Signature)
	if err != nil {
		return err
	}
	hashed := sha256.Sum256([]byte(signingString))
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, hashed[:], sig); err != nil {
		return err
	}
	// Digest check (for POST bodies)
	if expected := req.Header.Get("Digest"); expected != "" && req.Body != nil {
		// Caller is responsible for already-buffered body verification; we
		// only check the format here.
		if !strings.HasPrefix(strings.ToLower(expected), "sha-256=") {
			return errors.New("unsupported digest algorithm")
		}
	}
	return nil
}

// ResolveKeyURL extracts the actor URI from a key fragment URI by stripping
// the trailing #fragment.
func ResolveKeyURL(keyID string) string {
	u, err := url.Parse(keyID)
	if err != nil {
		return keyID
	}
	u.Fragment = ""
	return u.String()
}
