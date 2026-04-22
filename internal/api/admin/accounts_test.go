package admin_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- DeleteAllFilesOfUser ----------------------------------------------------

func TestDeleteAllFilesOfUser_DeletesOnlyTargetUserFiles(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	repo := testutil.NewMockDriveFileRepository()
	u1 := "u1"
	u2 := "u2"
	require.NoError(t, repo.Create(&model.DriveFile{ID: "f1", UserID: &u1}))
	require.NoError(t, repo.Create(&model.DriveFile{ID: "f2", UserID: &u1}))
	require.NoError(t, repo.Create(&model.DriveFile{ID: "f3", UserID: &u2}))
	h.SetDriveFileRepo(repo)

	rec := doPost(h.DeleteAllFilesOfUser, `{"userId":"u1"}`, adminUser)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.NotContains(t, repo.Files, "f1")
	assert.NotContains(t, repo.Files, "f2")
	assert.Contains(t, repo.Files, "f3", "other user's files must be kept")
}

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
