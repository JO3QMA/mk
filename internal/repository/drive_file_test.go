package repository

import (
	"context"
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func cleanupDriveFile(t *testing.T, id string) {
	t.Helper()
	testDB.Exec(`DELETE FROM "drive_file" WHERE id = ?`, id)
}

func newTestDriveFile(id, userID, md5 string, folderID *string) *model.DriveFile {
	uid := userID
	return &model.DriveFile{
		ID:             id,
		UserID:         &uid,
		MD5:            md5,
		Name:           "test.bin",
		Type:           "application/octet-stream",
		Size:           10,
		StoredInternal: true,
		URL:            "http://example.com/files/" + id,
		Properties:     datatypes.JSON([]byte("{}")),
		RequestHeaders: datatypes.JSON([]byte("{}")),
		FolderID:       folderID,
	}
}

func TestDriveFileRepository_CRUD(t *testing.T) {
	repo := NewDriveFileRepository(testDB)
	user := insertTestUser(t, "u_df_1", "dfu1")
	defer cleanupUser(t, user.ID)

	f := newTestDriveFile("f1", user.ID, "abc123", nil)
	require.NoError(t, repo.Create(f))
	defer cleanupDriveFile(t, f.ID)

	got, err := repo.FindByID("f1")
	require.NoError(t, err)
	assert.Equal(t, "test.bin", got.Name)

	require.NoError(t, repo.Update("f1", map[string]any{"name": "renamed.bin"}))
	got, _ = repo.FindByID("f1")
	assert.Equal(t, "renamed.bin", got.Name)

	require.NoError(t, repo.Update("f1", map[string]any{}))

	require.NoError(t, repo.Delete(f))
	_, err = repo.FindByID("f1")
	assert.Error(t, err)
}

func TestDriveFileRepository_FindByMD5(t *testing.T) {
	repo := NewDriveFileRepository(testDB)
	user := insertTestUser(t, "u_df_2", "dfu2")
	defer cleanupUser(t, user.ID)

	f1 := newTestDriveFile("f_md5_1", user.ID, "hash", nil)
	f2 := newTestDriveFile("f_md5_2", user.ID, "hash", nil)
	require.NoError(t, repo.Create(f1))
	require.NoError(t, repo.Create(f2))
	defer cleanupDriveFile(t, f1.ID)
	defer cleanupDriveFile(t, f2.ID)

	got, err := repo.FindByMD5(user.ID, "hash")
	require.NoError(t, err)
	// 最新 (id降順) を返す
	assert.Equal(t, "f_md5_2", got.ID)
}

func TestDriveFileRepository_ListByUser(t *testing.T) {
	repo := NewDriveFileRepository(testDB)
	folderRepo := NewDriveFolderRepository(testDB)
	user := insertTestUser(t, "u_df_3", "dfu3")
	defer cleanupUser(t, user.ID)

	uid := user.ID
	folder := &model.DriveFolder{ID: "fold_1", Name: "F", UserID: &uid}
	require.NoError(t, folderRepo.Create(folder))
	defer testDB.Exec(`DELETE FROM "drive_folder" WHERE id = ?`, folder.ID)

	folderID := "fold_1"
	root := newTestDriveFile("f_lst_1", user.ID, "h1", nil)
	infolder := newTestDriveFile("f_lst_2", user.ID, "h2", &folderID)
	require.NoError(t, repo.Create(root))
	require.NoError(t, repo.Create(infolder))
	defer cleanupDriveFile(t, root.ID)
	defer cleanupDriveFile(t, infolder.ID)

	rows, err := repo.ListByUser(user.ID, nil, "", "", 10)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "f_lst_1", rows[0].ID)

	rows, err = repo.ListByUser(user.ID, &folderID, "", "", 10)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "f_lst_2", rows[0].ID)

	// untilID/sinceIDの分岐を踏む (root のみ)
	rows, err = repo.ListByUser(user.ID, nil, "z", "", 10)
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	rows, err = repo.ListByUser(user.ID, nil, "", "a", 10)
	require.NoError(t, err)
	assert.Len(t, rows, 1)
}

func TestDriveFileRepository_QueryErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := NewDriveFileRepository(testDB.WithContext(ctx))

	err := repo.Create(&model.DriveFile{ID: "x"})
	assert.Error(t, err)
	_, err = repo.FindByID("a")
	assert.Error(t, err)
	_, err = repo.FindByMD5("a", "b")
	assert.Error(t, err)
	err = repo.Update("a", map[string]any{"name": "b"})
	assert.Error(t, err)
	_, err = repo.ListByUser("a", nil, "", "", 10)
	assert.Error(t, err)
}

func TestDriveFileRepository_ListForAdmin(t *testing.T) {
	repo := NewDriveFileRepository(testDB)
	user := insertTestUser(t, "u_admin_list", "adlf")
	defer cleanupUser(t, user.ID)

	localFile := newTestDriveFile("adl_local", user.ID, "md5local", nil)
	remoteHost := "remote.example"
	remoteFile := newTestDriveFile("adl_remote", user.ID, "md5remote", nil)
	remoteFile.UserHost = &remoteHost
	remoteFile.Type = "image/png"
	videoFile := newTestDriveFile("adl_video", user.ID, "md5video", nil)
	videoFile.Type = "video/mp4"
	require.NoError(t, repo.Create(localFile))
	require.NoError(t, repo.Create(remoteFile))
	require.NoError(t, repo.Create(videoFile))
	defer cleanupDriveFile(t, localFile.ID)
	defer cleanupDriveFile(t, remoteFile.ID)
	defer cleanupDriveFile(t, videoFile.ID)

	// origin = remote → only the remote-host file
	rows, err := repo.ListForAdmin("remote", "", "", "", "", 10)
	require.NoError(t, err)
	ids := make(map[string]bool)
	for _, r := range rows {
		ids[r.ID] = true
	}
	assert.True(t, ids[remoteFile.ID])
	assert.False(t, ids[localFile.ID])

	// type = image/ prefix
	rows, err = repo.ListForAdmin("", "", "image/", "", "", 10)
	require.NoError(t, err)
	ids = make(map[string]bool)
	for _, r := range rows {
		ids[r.ID] = true
	}
	assert.True(t, ids[remoteFile.ID])
	assert.False(t, ids[videoFile.ID])

	// host exact match
	rows, err = repo.ListForAdmin("", remoteHost, "", "", "", 10)
	require.NoError(t, err)
	found := false
	for _, r := range rows {
		if r.ID == remoteFile.ID {
			found = true
		}
	}
	assert.True(t, found)

	// default limit / clamp
	_, err = repo.ListForAdmin("", "", "", "", "", 0)
	require.NoError(t, err)
	_, err = repo.ListForAdmin("", "", "", "", "", 1000)
	require.NoError(t, err)
}

func TestDriveFileRepository_DeleteOrphansAndRemoteCache(t *testing.T) {
	repo := NewDriveFileRepository(testDB)
	user := insertTestUser(t, "u_del_src", "dels")
	defer cleanupUser(t, user.ID)

	// 孤立ファイル (userId NULL) と userId ありファイル
	orphan := newTestDriveFile("orph1", user.ID, "md5o", nil)
	orphan.UserID = nil
	kept := newTestDriveFile("kept1", user.ID, "md5k", nil)

	// remote cache (isLink=true, userHost set) と local link
	host := "cache.example"
	remoteCache := newTestDriveFile("rc1", user.ID, "md5rc", nil)
	remoteCache.IsLink = true
	remoteCache.UserHost = &host

	require.NoError(t, repo.Create(orphan))
	require.NoError(t, repo.Create(kept))
	require.NoError(t, repo.Create(remoteCache))
	defer cleanupDriveFile(t, orphan.ID)
	defer cleanupDriveFile(t, kept.ID)
	defer cleanupDriveFile(t, remoteCache.ID)

	n, err := repo.DeleteOrphans()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, int64(1))
	_, err = repo.FindByID(orphan.ID)
	assert.Error(t, err)
	_, err = repo.FindByID(kept.ID)
	assert.NoError(t, err)

	n, err = repo.DeleteRemoteCache()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, int64(1))
	_, err = repo.FindByID(remoteCache.ID)
	assert.Error(t, err)
}
