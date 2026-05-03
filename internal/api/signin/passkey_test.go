package signin_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/shiroha-a/mk/internal/api/signin"
	"github.com/shiroha-a/mk/internal/core/twofactor"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// /api/signin-with-passkey の init 段 (credential 無し) は challenge を含む
// PublicKeyCredentialRequestOptions と一回限りの context を返す (#705)。
func TestSigninWithPasskey_Init(t *testing.T) {
	if signinTestRedis == nil {
		t.Skip("redis testcontainer unavailable")
	}
	signinTestRedis.FlushAll(context.Background())
	h, _ := newTestHandler(t)
	svc, err := twofactor.NewWebAuthnService("https://example.com", "Misskey", signinTestRedis.Client)
	require.NoError(t, err)
	h.SetWebAuthn(svc, &inMemorySK{keys: map[string][]*model.UserSecurityKey{}})

	rec := doPost(h.SigninWithPasskey, `{}`)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	option, ok := resp["option"].(map[string]any)
	require.True(t, ok, "option must be a JSON object")
	// upstream 互換: option はそのまま PublicKeyCredentialRequestOptions
	// (no `publicKey` wrapper)。フロントは parseRequestOptionsFromJSON で読む。
	assert.NotEmpty(t, option["challenge"])
	assert.Nil(t, option["publicKey"])
	context, ok := resp["context"].(string)
	require.True(t, ok)
	assert.NotEmpty(t, context)
}

// init 2 回目: 別 context を返す (single-use)。
func TestSigninWithPasskey_InitTwice_DistinctContext(t *testing.T) {
	if signinTestRedis == nil {
		t.Skip("redis testcontainer unavailable")
	}
	signinTestRedis.FlushAll(context.Background())
	h, _ := newTestHandler(t)
	svc, err := twofactor.NewWebAuthnService("https://example.com", "Misskey", signinTestRedis.Client)
	require.NoError(t, err)
	h.SetWebAuthn(svc, &inMemorySK{keys: map[string][]*model.UserSecurityKey{}})

	rec1 := doPost(h.SigninWithPasskey, `{}`)
	rec2 := doPost(h.SigninWithPasskey, `{}`)
	var r1, r2 map[string]any
	require.NoError(t, json.Unmarshal(rec1.Body.Bytes(), &r1))
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &r2))
	assert.NotEqual(t, r1["context"], r2["context"])
}

// WebAuthnService が未注入のときは 503 を返す (frontend に「passkey 経由は
// 使えない」と伝える)。
func TestSigninWithPasskey_NotConfigured(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doPost(h.SigninWithPasskey, `{}`)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

// credential を送るが context が空の場合は 400 + 専用エラー ID。
func TestSigninWithPasskey_VerifyMissingContext(t *testing.T) {
	if signinTestRedis == nil {
		t.Skip("redis testcontainer unavailable")
	}
	signinTestRedis.FlushAll(context.Background())
	h, _ := newTestHandler(t)
	svc, err := twofactor.NewWebAuthnService("https://example.com", "Misskey", signinTestRedis.Client)
	require.NoError(t, err)
	h.SetWebAuthn(svc, &inMemorySK{keys: map[string][]*model.UserSecurityKey{}})

	body := `{"credential":{"id":"x","rawId":"x","type":"public-key","response":{}}}`
	rec := doPost(h.SigninWithPasskey, body)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	errMap, ok := resp["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "1658cc2e-4495-461f-aee4-d403cdf073c1", errMap["id"])
}

// 不正な credential body は signature verify が走る前に 403 を返す。
func TestSigninWithPasskey_VerifyBadCredential(t *testing.T) {
	if signinTestRedis == nil {
		t.Skip("redis testcontainer unavailable")
	}
	signinTestRedis.FlushAll(context.Background())
	h, _ := newTestHandler(t)
	svc, err := twofactor.NewWebAuthnService("https://example.com", "Misskey", signinTestRedis.Client)
	require.NoError(t, err)
	h.SetWebAuthn(svc, &inMemorySK{keys: map[string][]*model.UserSecurityKey{}})

	body := `{"context":"ghost","credential":{"id":"x","rawId":"x","type":"public-key","response":{}}}`
	rec := doPost(h.SigninWithPasskey, body)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// JSON body が壊れている場合は 400。
func TestSigninWithPasskey_BadBody(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doPost(h.SigninWithPasskey, `not-json`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// Step 1 中に entropy 枯渇 → 500。readRandom seam を差し替える。
func TestSigninWithPasskey_NewContextRandError(t *testing.T) {
	if signinTestRedis == nil {
		t.Skip("redis testcontainer unavailable")
	}
	signinTestRedis.FlushAll(context.Background())
	h, _ := newTestHandler(t)
	svc, err := twofactor.NewWebAuthnService("https://example.com", "Misskey", signinTestRedis.Client)
	require.NoError(t, err)
	h.SetWebAuthn(svc, &inMemorySK{keys: map[string][]*model.UserSecurityKey{}})

	old := signin.SwapReadRandomBytes(func(_ []byte) (int, error) { return 0, assert.AnError })
	defer signin.SwapReadRandomBytes(old)

	rec := doPost(h.SigninWithPasskey, `{}`)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// passwordless login が有効でないユーザーは認証成功しても 403 を返す
// (resolver が user を返してきても profile.usePasswordLessLogin が
// false の場合)。
//
// この path は実 webauthn 検証を通せないので、resolver 経由までは到達するが
// 必ず先に signature verify で fail する。互換性確認のための smoke のみ。
func TestSigninWithPasskey_PasswordlessNotEnabled_Fails(t *testing.T) {
	if signinTestRedis == nil {
		t.Skip("redis testcontainer unavailable")
	}
	signinTestRedis.FlushAll(context.Background())
	h, repo := newTestHandler(t)

	svc, err := twofactor.NewWebAuthnService("https://example.com", "Misskey", signinTestRedis.Client)
	require.NoError(t, err)
	h.SetWebAuthn(svc, &inMemorySK{keys: map[string][]*model.UserSecurityKey{}})

	// usePasswordLessLogin=false のユーザー
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.MinCost)
	hashStr := string(hash)
	repo.Users["uPLN"] = &model.User{ID: "uPLN", Username: "pln", UsernameLower: "pln"}
	repo.Profiles["uPLN"] = &model.UserProfile{
		UserID:               "uPLN",
		Password:             &hashStr,
		UsePasswordLessLogin: false,
	}

	body := `{"context":"ghost","credential":{"id":"x","rawId":"x","type":"public-key","response":{}}}`
	rec := doPost(h.SigninWithPasskey, body)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}
