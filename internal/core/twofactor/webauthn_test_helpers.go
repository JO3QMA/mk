package twofactor

import (
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

// makeFakeSessionData builds a minimal SessionData for round-trip serialization
// tests. The Challenge string is the only field these tests inspect.
func makeFakeSessionData() *webauthn.SessionData {
	return &webauthn.SessionData{
		Challenge:      "test-challenge",
		RelyingPartyID: "example.com",
		UserID:         []byte("alice"),
	}
}

// makeFakeCredential builds a minimal *webauthn.Credential for tests of
// CredentialToModel — the function only reads ID / PublicKey / Authenticator
// / Transport, so we can leave the rest zero-valued.
func makeFakeCredential(id, pub []byte, signCount uint32, transports []string) *webauthn.Credential {
	c := &webauthn.Credential{
		ID:        id,
		PublicKey: pub,
		Authenticator: webauthn.Authenticator{
			SignCount: signCount,
		},
	}
	for _, t := range transports {
		c.Transport = append(c.Transport, protocol.AuthenticatorTransport(t))
	}
	return c
}
