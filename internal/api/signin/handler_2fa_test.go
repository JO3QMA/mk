package signin_test

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/pquerna/otp/totp"
	"github.com/shiroha-a/mk/internal/api/signin"
	"github.com/shiroha-a/mk/internal/core/twofactor"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

var signinTestRedis *testutil.TestRedis

func TestMain(m *testing.M) {
	ctx := context.Background()
	tr, err := testutil.SetupRedis(ctx)
	if err != nil {
		log.Printf("api/signin: redis testcontainer unavailable: %v", err)
		os.Exit(m.Run())
	}
	signinTestRedis = tr
	code := m.Run()
	signinTestRedis.Teardown(ctx)
	os.Exit(code)
}

// helpers

func newTestUserWithTOTP(repo *testutil.MockUserRepository, username, password, totpSecret string, backupCodes []string) *model.User {
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	hashStr := string(hash)
	token := "testtoken1234567"
	user := &model.User{
		ID:            "u1",
		Username:      username,
		UsernameLower: strings.ToLower(username),
		Token:         &token,
	}
	repo.Users["u1"] = user
	prof := &model.UserProfile{
		UserID:           "u1",
		Password:         &hashStr,
		TwoFactorEnabled: true,
		TwoFactorSecret:  &totpSecret,
	}
	if backupCodes != nil {
		prof.TwoFactorBackupSecret = pq.StringArray(backupCodes)
	}
	repo.Profiles["u1"] = prof
	return user
}

// inMemorySK is a no-DB implementation of UserSecurityKeyRepository for the
// signin tests that exercise the WebAuthn assertion path. We only need
// ListByUser + UpdateCounter; everything else is no-op.
type inMemorySK struct {
	keys map[string][]*model.UserSecurityKey
}

func (r *inMemorySK) Create(_ *model.UserSecurityKey) error { return nil }
func (r *inMemorySK) FindByID(_ string) (*model.UserSecurityKey, error) {
	return nil, testutil.ErrNotFound
}
func (r *inMemorySK) ListByUser(userID string) ([]*model.UserSecurityKey, error) {
	return r.keys[userID], nil
}
func (r *inMemorySK) UpdateName(_, _, _ string) error       { return nil }
func (r *inMemorySK) UpdateCounter(_ string, _ int64) error { return nil }
func (r *inMemorySK) Delete(_, _ string) error              { return nil }
func (r *inMemorySK) CountByUser(_ string) (int64, error)   { return 0, nil }

var _ repository.UserSecurityKeyRepository = (*inMemorySK)(nil)

// --- TOTP ---

func Test2FA_TOTP_Step3_Success(t *testing.T) {
	h, repo := newTestHandler(t)
	// 既知の TOTP secret を使う
	secret := "JBSWY3DPEHPK3PXP" // base32 "Hello!"
	newTestUserWithTOTP(repo, "alice", "pass", secret, nil)

	// Step 2: パスワードを送ると 2FA が必要と告げられる
	rec := doPost(h.SigninFlow, `{"username":"alice","password":"pass"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	var step2 map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &step2))
	assert.Equal(t, false, step2["finished"])
	assert.Equal(t, "totp", step2["next"])

	// Step 3: 有効な TOTP token を送ると finished
	token, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err)
	rec2 := doPost(h.SigninFlow, `{"username":"alice","password":"pass","token":"`+token+`"}`)
	require.Equal(t, http.StatusOK, rec2.Code)
	var step3 map[string]any
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &step3))
	assert.Equal(t, true, step3["finished"])
}

func Test2FA_TOTP_InvalidToken(t *testing.T) {
	h, repo := newTestHandler(t)
	newTestUserWithTOTP(repo, "alice", "pass", "JBSWY3DPEHPK3PXP", nil)
	rec := doPost(h.SigninFlow, `{"username":"alice","password":"pass","token":"000000"}`)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// --- Backup codes ---

func Test2FA_BackupCode_Success(t *testing.T) {
	h, repo := newTestHandler(t)
	codes := []string{"abcd1234", "efgh5678"}
	newTestUserWithTOTP(repo, "alice", "pass", "JBSWY3DPEHPK3PXP", codes)

	rec := doPost(h.SigninFlow, `{"username":"alice","password":"pass","token":"abcd1234"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["finished"])

	// 使用済みコードが消えている (single-use)
	remaining := repo.Profiles["u1"].TwoFactorBackupSecret
	assert.Equal(t, []string{"efgh5678"}, []string(remaining))
}

func Test2FA_BackupCode_AllInvalid(t *testing.T) {
	h, repo := newTestHandler(t)
	newTestUserWithTOTP(repo, "alice", "pass", "JBSWY3DPEHPK3PXP", []string{"valid"})
	rec := doPost(h.SigninFlow, `{"username":"alice","password":"pass","token":"wrong"}`)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// --- WebAuthn assertion (no keys → fallback to TOTP) ---

func Test2FA_WebAuthnNoKeys_FallsBackToTOTP(t *testing.T) {
	h, repo := newTestHandler(t)
	if signinTestRedis == nil {
		t.Skip("redis testcontainer unavailable")
	}
	svc, err := twofactor.NewWebAuthnService("https://example.com", "Misskey", signinTestRedis.Client)
	require.NoError(t, err)
	skRepo := &inMemorySK{keys: map[string][]*model.UserSecurityKey{}}
	h.SetWebAuthn(svc, skRepo)

	newTestUserWithTOTP(repo, "alice", "pass", "JBSWY3DPEHPK3PXP", nil)
	// パスワードのみ → TOTP step を返す (security key 無し)
	rec := doPost(h.SigninFlow, `{"username":"alice","password":"pass"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "totp", resp["next"])
}

func Test2FA_WebAuthnWithKeys_ReturnsAssertionChallenge(t *testing.T) {
	h, repo := newTestHandler(t)
	if signinTestRedis == nil {
		t.Skip("redis testcontainer unavailable")
	}
	signinTestRedis.FlushAll(context.Background())
	svc, err := twofactor.NewWebAuthnService("https://example.com", "Misskey", signinTestRedis.Client)
	require.NoError(t, err)
	skRepo := &inMemorySK{
		keys: map[string][]*model.UserSecurityKey{
			"u1": {{ID: "AAEC", PublicKey: "AwQF", UserID: "u1"}},
		},
	}
	h.SetWebAuthn(svc, skRepo)

	newTestUserWithTOTP(repo, "alice", "pass", "JBSWY3DPEHPK3PXP", nil)
	rec := doPost(h.SigninFlow, `{"username":"alice","password":"pass"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "captcha-keys", resp["next"])
	assert.NotEmpty(t, resp["sessionId"])
	assert.NotNil(t, resp["assertion"])
}

func Test2FA_WebAuthnFinishLogin_BadCredential(t *testing.T) {
	h, repo := newTestHandler(t)
	if signinTestRedis == nil {
		t.Skip("redis testcontainer unavailable")
	}
	svc, err := twofactor.NewWebAuthnService("https://example.com", "Misskey", signinTestRedis.Client)
	require.NoError(t, err)
	skRepo := &inMemorySK{
		keys: map[string][]*model.UserSecurityKey{
			"u1": {{ID: "AAEC", PublicKey: "AwQF", UserID: "u1"}},
		},
	}
	h.SetWebAuthn(svc, skRepo)
	newTestUserWithTOTP(repo, "alice", "pass", "JBSWY3DPEHPK3PXP", nil)

	// 存在しない sessionId + 空 credential → 403
	rec := doPost(h.SigninFlow, `{"username":"alice","password":"pass","sessionId":"ghost","credential":{"id":"x","rawId":"x","type":"public-key","response":{}}}`)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// 静的: signin.Handler の使用を保証する
var _ = signin.NewHandler
