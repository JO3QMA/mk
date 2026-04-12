package i

import (
	"context"
	"log"
	"net/http"
	"os"
	"testing"

	"github.com/shiroha-a/mk/internal/core/twofactor"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// この test file は WebAuthn 関連 5 ハンドラの validation / wiring を網羅する。
// FinishRegistration / FinishLogin の本物の attestation 検証は
// internal/core/twofactor の test に任せる (W3C spec vector を使用)。
// ここでは:
//   - パラメータ検証 (空 body / password 欠落)
//   - 認証チェック (wrong password / no profile)
//   - WebAuthn 未注入時の 503
//   - register-key / remove-key / update-key / password-less の正常系 wiring
//
// 共通: TestMain で testcontainers の Redis を立ち上げて WebAuthnService を
// 注入できる状態にしておく。

var iTestRedis *testutil.TestRedis

func TestMain(m *testing.M) {
	ctx := context.Background()
	tr, err := testutil.SetupRedis(ctx)
	if err != nil {
		log.Printf("api/i: redis testcontainer unavailable: %v", err)
		os.Exit(m.Run())
	}
	iTestRedis = tr
	code := m.Run()
	iTestRedis.Teardown(ctx)
	os.Exit(code)
}

// inMemorySecurityKeyRepo は最小限の repository.UserSecurityKeyRepository 実装。
// DB を立てずに handler のロジックを exercise したい用途。
type inMemorySecurityKeyRepo struct {
	keys map[string]*model.UserSecurityKey
}

func newInMemSKRepo() *inMemorySecurityKeyRepo {
	return &inMemorySecurityKeyRepo{keys: map[string]*model.UserSecurityKey{}}
}

func (r *inMemorySecurityKeyRepo) Create(k *model.UserSecurityKey) error {
	r.keys[k.ID] = k
	return nil
}

func (r *inMemorySecurityKeyRepo) FindByID(id string) (*model.UserSecurityKey, error) {
	if k, ok := r.keys[id]; ok {
		return k, nil
	}
	return nil, testutil.ErrNotFound
}

func (r *inMemorySecurityKeyRepo) ListByUser(userID string) ([]*model.UserSecurityKey, error) {
	var out []*model.UserSecurityKey
	for _, k := range r.keys {
		if k.UserID == userID {
			out = append(out, k)
		}
	}
	return out, nil
}

func (r *inMemorySecurityKeyRepo) UpdateName(id, userID, name string) error {
	k, ok := r.keys[id]
	if !ok || k.UserID != userID {
		return testutil.ErrNotFound
	}
	k.Name = name
	return nil
}

func (r *inMemorySecurityKeyRepo) UpdateCounter(id string, counter int64) error {
	k, ok := r.keys[id]
	if !ok {
		return testutil.ErrNotFound
	}
	k.Counter = counter
	return nil
}

func (r *inMemorySecurityKeyRepo) Delete(id, userID string) error {
	k, ok := r.keys[id]
	if !ok || k.UserID != userID {
		return testutil.ErrNotFound
	}
	delete(r.keys, id)
	return nil
}

func (r *inMemorySecurityKeyRepo) CountByUser(userID string) (int64, error) {
	var n int64
	for _, k := range r.keys {
		if k.UserID == userID {
			n++
		}
	}
	return n, nil
}

// 静的に interface を満たすことを確認
var _ repository.UserSecurityKeyRepository = (*inMemorySecurityKeyRepo)(nil)

// newWebAuthnHandler builds an extra Handler with WebAuthn dependencies wired
// up against the testcontainer Redis instance.
func newWebAuthnHandler(t *testing.T) (*Handler, *testutil.MockUserRepository, *inMemorySecurityKeyRepo) {
	t.Helper()
	if iTestRedis == nil {
		t.Skip("redis testcontainer not available")
	}
	h, repo := newExtraHandler(t)
	skRepo := newInMemSKRepo()
	svc, err := twofactor.NewWebAuthnService("https://example.com", "Misskey", iTestRedis.Client)
	require.NoError(t, err)
	h.SetWebAuthn(svc, skRepo)
	return h, repo, skRepo
}

// --- TwoFARegisterKey ---

func TestTwoFARegisterKey_NotConfigured(t *testing.T) {
	h, repo := newExtraHandler(t)
	user := setupUserWithPassword(repo, "u1", "pass")
	rec := postExtra(h.TwoFARegisterKey, `{"password":"pass"}`, user)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestTwoFARegisterKey_NoPassword(t *testing.T) {
	h, _, _ := newWebAuthnHandler(t)
	rec := postExtra(h.TwoFARegisterKey, `{}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestTwoFARegisterKey_WrongPassword(t *testing.T) {
	h, repo, _ := newWebAuthnHandler(t)
	user := setupUserWithPassword(repo, "u1", "correct")
	rec := postExtra(h.TwoFARegisterKey, `{"password":"wrong"}`, user)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestTwoFARegisterKey_NoProfile(t *testing.T) {
	h, repo, _ := newWebAuthnHandler(t)
	repo.Users["u1"] = &model.User{ID: "u1", Username: "u1"}
	rec := postExtra(h.TwoFARegisterKey, `{"password":"pass"}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestTwoFARegisterKey_Success(t *testing.T) {
	h, repo, _ := newWebAuthnHandler(t)
	user := setupUserWithPassword(repo, "u1", "pass")
	rec := postExtra(h.TwoFARegisterKey, `{"password":"pass"}`, user)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "sessionId")
	assert.Contains(t, rec.Body.String(), "creation")
}

// --- TwoFAKeyDone ---

func TestTwoFAKeyDone_MissingFields(t *testing.T) {
	h, repo, _ := newWebAuthnHandler(t)
	user := setupUserWithPassword(repo, "u1", "pass")
	rec := postExtra(h.TwoFAKeyDone, `{"password":"pass"}`, user)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestTwoFAKeyDone_WrongPassword(t *testing.T) {
	h, repo, _ := newWebAuthnHandler(t)
	user := setupUserWithPassword(repo, "u1", "correct")
	rec := postExtra(h.TwoFAKeyDone, `{"password":"wrong","sessionId":"s","response":{}}`, user)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestTwoFAKeyDone_FailureBranch(t *testing.T) {
	h, repo, _ := newWebAuthnHandler(t)
	user := setupUserWithPassword(repo, "u1", "pass")
	// 存在しない sessionId + 不正な response → FinishRegistration が失敗 → 403
	rec := postExtra(h.TwoFAKeyDone, `{"password":"pass","sessionId":"ghost","response":{"id":"x","rawId":"x","type":"public-key","response":{"attestationObject":"","clientDataJSON":""}}}`, user)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// --- TwoFARemoveKey ---

func TestTwoFARemoveKey_MissingFields(t *testing.T) {
	h, repo, _ := newWebAuthnHandler(t)
	user := setupUserWithPassword(repo, "u1", "pass")
	rec := postExtra(h.TwoFARemoveKey, `{"password":"pass"}`, user)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestTwoFARemoveKey_NotFound(t *testing.T) {
	h, repo, _ := newWebAuthnHandler(t)
	user := setupUserWithPassword(repo, "u1", "pass")
	rec := postExtra(h.TwoFARemoveKey, `{"password":"pass","credentialId":"ghost"}`, user)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestTwoFARemoveKey_Success(t *testing.T) {
	h, repo, skRepo := newWebAuthnHandler(t)
	user := setupUserWithPassword(repo, "u1", "pass")
	require.NoError(t, skRepo.Create(&model.UserSecurityKey{
		ID: "key1", UserID: "u1", Name: "k", PublicKey: "pk",
	}))
	rec := postExtra(h.TwoFARemoveKey, `{"password":"pass","credentialId":"key1"}`, user)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	// 残り 0 件なので securityKeysAvailable=false にされる
	assert.False(t, repo.Profiles["u1"].SecurityKeysAvailable)
	assert.False(t, repo.Profiles["u1"].UsePasswordLessLogin)
}

// --- TwoFAUpdateKey ---

func TestTwoFAUpdateKey_MissingFields(t *testing.T) {
	h, repo, _ := newWebAuthnHandler(t)
	user := setupUserWithPassword(repo, "u1", "pass")
	rec := postExtra(h.TwoFAUpdateKey, `{"password":"pass","credentialId":"k"}`, user)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestTwoFAUpdateKey_NotFound(t *testing.T) {
	h, repo, _ := newWebAuthnHandler(t)
	user := setupUserWithPassword(repo, "u1", "pass")
	rec := postExtra(h.TwoFAUpdateKey, `{"password":"pass","credentialId":"ghost","name":"x"}`, user)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestTwoFAUpdateKey_Success(t *testing.T) {
	h, repo, skRepo := newWebAuthnHandler(t)
	user := setupUserWithPassword(repo, "u1", "pass")
	require.NoError(t, skRepo.Create(&model.UserSecurityKey{
		ID: "key1", UserID: "u1", Name: "old", PublicKey: "pk",
	}))
	rec := postExtra(h.TwoFAUpdateKey, `{"password":"pass","credentialId":"key1","name":"renamed"}`, user)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "renamed", skRepo.keys["key1"].Name)
}

// --- TwoFAPasswordLess ---

func TestTwoFAPasswordLess_NoPassword(t *testing.T) {
	h, _, _ := newWebAuthnHandler(t)
	rec := postExtra(h.TwoFAPasswordLess, `{}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestTwoFAPasswordLess_EnableNoKeys(t *testing.T) {
	h, repo, _ := newWebAuthnHandler(t)
	user := setupUserWithPassword(repo, "u1", "pass")
	rec := postExtra(h.TwoFAPasswordLess, `{"password":"pass","value":true}`, user)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestTwoFAPasswordLess_EnableWithKey(t *testing.T) {
	h, repo, skRepo := newWebAuthnHandler(t)
	user := setupUserWithPassword(repo, "u1", "pass")
	require.NoError(t, skRepo.Create(&model.UserSecurityKey{ID: "k1", UserID: "u1"}))
	rec := postExtra(h.TwoFAPasswordLess, `{"password":"pass","value":true}`, user)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.True(t, repo.Profiles["u1"].UsePasswordLessLogin)
}

func TestTwoFAPasswordLess_Disable(t *testing.T) {
	h, repo, _ := newWebAuthnHandler(t)
	user := setupUserWithPassword(repo, "u1", "pass")
	repo.Profiles["u1"].UsePasswordLessLogin = true
	rec := postExtra(h.TwoFAPasswordLess, `{"password":"pass","value":false}`, user)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.False(t, repo.Profiles["u1"].UsePasswordLessLogin)
}
