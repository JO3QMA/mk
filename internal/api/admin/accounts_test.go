package admin_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- UpdateProxyAccount ------------------------------------------------------

// stubSystemAccountFetcher is a minimal SystemAccountFetcher returning a
// pre-configured user for any kind.
type stubSystemAccountFetcher struct {
	user *model.User
	err  error
}

func (s *stubSystemAccountFetcher) Fetch(_ string) (*model.User, error) {
	return s.user, s.err
}

func TestUpdateProxyAccount_UpdatesDescription(t *testing.T) {
	h, userRepo, _, _ := newTestHandler(t)
	proxy := &model.User{ID: "u-proxy", Username: "proxy.actor", UsernameLower: "proxy.actor"}
	require.NoError(t, userRepo.Create(proxy))
	require.NoError(t, userRepo.CreateProfile(&model.UserProfile{UserID: proxy.ID}))
	h.SetSystemAccountFetcher(&stubSystemAccountFetcher{user: proxy})

	rec := doPost(h.UpdateProxyAccount, `{"description":"hello"}`, adminUser)
	require.Equal(t, http.StatusOK, rec.Code)
	prof, err := userRepo.FindProfileByUserID(proxy.ID)
	require.NoError(t, err)
	require.NotNil(t, prof.Description)
	assert.Equal(t, "hello", *prof.Description)
}

func TestUpdateProxyAccount_NoFetcherIsNoOp(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.UpdateProxyAccount, `{"description":"x"}`, adminUser)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestUpdateProxyAccount_FetcherErrorReturnsInternal(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetSystemAccountFetcher(&stubSystemAccountFetcher{err: errors.New("boom")})
	rec := doPost(h.UpdateProxyAccount, `{"description":"x"}`, adminUser)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// description が undefined (null/省略) のときは profile を更新しないが、
// systemAccountFetcher 経由で取得した user は返す。
func TestUpdateProxyAccount_NoDescriptionDoesNotUpdateProfile(t *testing.T) {
	h, userRepo, _, _ := newTestHandler(t)
	proxy := &model.User{ID: "u-noop", Username: "proxy.actor", UsernameLower: "proxy.actor"}
	require.NoError(t, userRepo.Create(proxy))
	require.NoError(t, userRepo.CreateProfile(&model.UserProfile{UserID: proxy.ID}))
	h.SetSystemAccountFetcher(&stubSystemAccountFetcher{user: proxy})

	rec := doPost(h.UpdateProxyAccount, `{}`, adminUser)
	require.Equal(t, http.StatusOK, rec.Code)
	prof, err := userRepo.FindProfileByUserID(proxy.ID)
	require.NoError(t, err)
	assert.Nil(t, prof.Description)
}

// --- /admin/accounts/delete + /admin/delete-account (#574 で stub から
// 移行した本実装テスト) ---

// stubDeleteAccountEnqueuer captures EnqueueDeleteAccount payloads so
// tests can assert that AccountsDelete / DeleteAccount schedule cascade.
type stubDeleteAccountEnqueuer struct {
	lastUserID string
	called     int
	err        error
}

func (s *stubDeleteAccountEnqueuer) EnqueueDeleteAccount(payload queue.DeleteAccountPayload) error {
	s.called++
	s.lastUserID = payload.UserID
	return s.err
}

func TestAccountsDelete(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.AccountsDelete, `{}`, adminUser).Code)
}

func TestAccountsDelete_EnqueuesCascade(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	stub := &stubDeleteAccountEnqueuer{}
	h.SetDeleteAccountEnqueuer(stub)
	assert.Equal(t, http.StatusNoContent,
		doPost(h.AccountsDelete, `{"userId":"u1"}`, adminUser).Code)
	assert.Equal(t, 1, stub.called)
	assert.Equal(t, "u1", stub.lastUserID)
}

func TestAccountsDelete_MissingUserIDSkipsEnqueue(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	stub := &stubDeleteAccountEnqueuer{}
	h.SetDeleteAccountEnqueuer(stub)
	assert.Equal(t, http.StatusNoContent,
		doPost(h.AccountsDelete, `{}`, adminUser).Code)
	assert.Equal(t, 0, stub.called)
}

func TestAccountsDelete_EnqueueFailureIsLogged(t *testing.T) {
	// enqueue 失敗はログに残すだけで HTTP 応答は 204 のまま返ることを確認。
	h, _, _, _ := newTestHandler(t)
	h.SetDeleteAccountEnqueuer(&stubDeleteAccountEnqueuer{err: errors.New("boom")})
	assert.Equal(t, http.StatusNoContent,
		doPost(h.AccountsDelete, `{"userId":"u1"}`, adminUser).Code)
}

func TestDeleteAccountAdmin(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.DeleteAccount, `{}`, adminUser).Code)
}

func TestDeleteAccount_EnqueuesCascade(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	stub := &stubDeleteAccountEnqueuer{}
	h.SetDeleteAccountEnqueuer(stub)
	assert.Equal(t, http.StatusNoContent,
		doPost(h.DeleteAccount, `{"userId":"u9"}`, adminUser).Code)
	assert.Equal(t, "u9", stub.lastUserID)
}

// --- /admin/accounts/find-by-email ---

func TestAccountsFindByEmail_Empty(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusBadRequest, doPost(h.AccountsFindByEmail, `{}`, adminUser).Code)
}

func TestAccountsFindByEmail_NotFound(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNotFound, doPost(h.AccountsFindByEmail, `{"email":"ghost@example.com"}`, adminUser).Code)
}

func TestAccountsFindByEmail_Found(t *testing.T) {
	h, userRepo, _, _ := newTestHandler(t)
	email := "alice@example.com"
	userRepo.Users["alice"] = &model.User{ID: "alice", Username: "alice"}
	userRepo.Profiles["alice"] = &model.UserProfile{UserID: "alice", Email: &email}

	rec := doPost(h.AccountsFindByEmail, `{"email":"alice@example.com"}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"alice"`)
}

// TestAccountsFindByEmail_ResponseShape verifies that AccountsFindByEmail
// returns the packAdminUser format: frontend-expected fields (createdAt,
// roles, policies) must be present, and internal model fields (inbox,
// sharedInbox, usernameLower) must NOT leak.
func TestAccountsFindByEmail_ResponseShape(t *testing.T) {
	h, userRepo, _, _ := newTestHandler(t)

	// aidx で生成した ID を使い、createdAt がパースされることを保証する
	idGen, _ := id.NewGenerator("aidx")
	uid := idGen.Generate(time.Now())

	inbox := "https://remote.example/inbox"
	sharedInbox := "https://remote.example/sharedInbox"
	email := "bob@example.com"
	userRepo.Users[uid] = &model.User{
		ID:                uid,
		Username:          "bob",
		UsernameLower:     "bob",
		Inbox:             &inbox,
		SharedInbox:       &sharedInbox,
		IsExplorable:      true,
		AvatarDecorations: []byte("[]"),
	}
	userRepo.Profiles[uid] = &model.UserProfile{
		UserID:          uid,
		Email:           &email,
		MutedWords:      []byte("[]"),
		HardMutedWords:  []byte("[]"),
		MutedInstances:  []byte("[]"),
		PublicReactions: true,
	}

	rec := doPost(h.AccountsFindByEmail, `{"email":"bob@example.com"}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	// packAdminUser が付与する frontend 必須フィールドの存在確認
	assert.Equal(t, uid, resp["id"])
	assert.NotNil(t, resp["createdAt"], "createdAt must be present")
	assert.NotNil(t, resp["roles"], "roles must be present")
	assert.NotNil(t, resp["policies"], "policies must be present")

	// 内部フィールドが response に漏れていないことを確認
	_, hasInbox := resp["inbox"]
	assert.False(t, hasInbox, "inbox is an internal field and must not be exposed")
	_, hasSharedInbox := resp["sharedInbox"]
	assert.False(t, hasSharedInbox, "sharedInbox is an internal field and must not be exposed")
	_, hasUsernameLower := resp["usernameLower"]
	assert.False(t, hasUsernameLower, "usernameLower is an internal field and must not be exposed")
}
