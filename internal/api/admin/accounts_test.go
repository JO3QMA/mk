package admin_test

import (
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

func TestUpdateProxyAccount_SetsProxyAccountID(t *testing.T) {
	h, userRepo, metaRepo, _ := newTestHandler(t)
	u := &model.User{ID: "u-proxy", Username: "proxybot", UsernameLower: "proxybot"}
	require.NoError(t, userRepo.Create(u))

	rec := doPost(h.UpdateProxyAccount, `{"username":"proxybot"}`, adminUser)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	require.NotNil(t, metaRepo.Meta.ProxyAccountID)
	assert.Equal(t, "u-proxy", *metaRepo.Meta.ProxyAccountID)
}

func TestUpdateProxyAccount_ClearWithNilUsername(t *testing.T) {
	h, _, metaRepo, _ := newTestHandler(t)
	existing := "old-proxy"
	metaRepo.Meta.ProxyAccountID = &existing

	rec := doPost(h.UpdateProxyAccount, `{"username":null}`, adminUser)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Nil(t, metaRepo.Meta.ProxyAccountID)
}

func TestUpdateProxyAccount_ClearWithEmptyString(t *testing.T) {
	h, _, metaRepo, _ := newTestHandler(t)
	existing := "old-proxy"
	metaRepo.Meta.ProxyAccountID = &existing

	rec := doPost(h.UpdateProxyAccount, `{"username":""}`, adminUser)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Nil(t, metaRepo.Meta.ProxyAccountID)
}

func TestUpdateProxyAccount_UnknownUsernameReturns404(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.UpdateProxyAccount, `{"username":"ghost"}`, adminUser)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUpdateProxyAccount_UsernameIsLowercased(t *testing.T) {
	h, userRepo, metaRepo, _ := newTestHandler(t)
	u := &model.User{ID: "u-case", Username: "MixedCase", UsernameLower: "mixedcase"}
	require.NoError(t, userRepo.Create(u))

	rec := doPost(h.UpdateProxyAccount, `{"username":"MixedCase"}`, adminUser)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	require.NotNil(t, metaRepo.Meta.ProxyAccountID)
	assert.Equal(t, "u-case", *metaRepo.Meta.ProxyAccountID)
}
