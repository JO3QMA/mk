package drive_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/shiroha-a/mk/internal/core/drive"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var stubError = errors.New("stub error")

// brokenStorage points the LocalStorage rootDir at an existing file so that
// any Put/Delete operation fails.
func brokenStorage(t *testing.T) *drive.LocalStorage {
	t.Helper()
	tmp := t.TempDir()
	conflict := filepath.Join(tmp, "conflict")
	require.NoError(t, os.WriteFile(conflict, []byte("x"), 0o644))
	return drive.NewLocalStorage(conflict, "")
}

func newSvc(t *testing.T) (*drive.Service, *testutil.MockDriveFileRepository, *testutil.MockDriveFolderRepository) {
	t.Helper()
	fileRepo := testutil.NewMockDriveFileRepository()
	folderRepo := testutil.NewMockDriveFolderRepository()
	folderRepo.FilesRef = fileRepo
	storage := drive.NewLocalStorage(t.TempDir(), "https://example.com/files")
	idGen, _ := id.NewGenerator("aidx")
	svc := drive.NewService(fileRepo, folderRepo, storage, idGen)
	return svc, fileRepo, folderRepo
}

// --- Upload ---

func TestUpload_NilUser(t *testing.T) {
	svc, _, _ := newSvc(t)
	_, err := svc.Upload(drive.UploadInput{Body: []byte("x")})
	assert.Error(t, err)
}

func TestUpload_HappyPath(t *testing.T) {
	svc, fileRepo, _ := newSvc(t)
	user := &model.User{ID: "u1"}
	f, err := svc.Upload(drive.UploadInput{User: user, Body: []byte("hello"), Name: "hello.txt"})
	require.NoError(t, err)
	assert.Equal(t, "hello.txt", f.Name)
	assert.Len(t, fileRepo.Files, 1)
}

func TestUpload_DedupByMD5(t *testing.T) {
	svc, fileRepo, _ := newSvc(t)
	user := &model.User{ID: "u1"}
	first, _ := svc.Upload(drive.UploadInput{User: user, Body: []byte("data"), Name: "a"})
	second, _ := svc.Upload(drive.UploadInput{User: user, Body: []byte("data"), Name: "b"})
	assert.Equal(t, first.ID, second.ID)
	assert.Len(t, fileRepo.Files, 1)
}

func TestUpload_ForceSkipsDedup(t *testing.T) {
	svc, fileRepo, _ := newSvc(t)
	user := &model.User{ID: "u1"}
	_, err := svc.Upload(drive.UploadInput{User: user, Body: []byte("d"), Name: "a"})
	require.NoError(t, err)
	_, err = svc.Upload(drive.UploadInput{User: user, Body: []byte("d"), Name: "a", Force: true})
	require.NoError(t, err)
	assert.Len(t, fileRepo.Files, 2)
}

func TestUpload_FolderNotFound(t *testing.T) {
	svc, _, _ := newSvc(t)
	folderID := "ghost"
	_, err := svc.Upload(drive.UploadInput{User: &model.User{ID: "u1"}, Body: []byte("x"), FolderID: &folderID})
	require.ErrorIs(t, err, drive.ErrFolderNotFound)
}

func TestUpload_FolderAccessDenied(t *testing.T) {
	svc, _, folderRepo := newSvc(t)
	other := "other"
	folderRepo.Folders["fid"] = &model.DriveFolder{ID: "fid", UserID: &other}
	folderID := "fid"
	_, err := svc.Upload(drive.UploadInput{User: &model.User{ID: "u1"}, Body: []byte("x"), FolderID: &folderID})
	require.ErrorIs(t, err, drive.ErrAccessDenied)
}

// failingFileRepo causes Create to fail.
type failingFileRepo struct {
	*testutil.MockDriveFileRepository
}

func (f *failingFileRepo) Create(_ *model.DriveFile) error {
	return stubError
}

func TestUpload_RepoCreateError(t *testing.T) {
	repo := &failingFileRepo{MockDriveFileRepository: testutil.NewMockDriveFileRepository()}
	idGen, _ := id.NewGenerator("aidx")
	svc := drive.NewService(repo, testutil.NewMockDriveFolderRepository(), drive.NewLocalStorage(t.TempDir(), ""), idGen)
	_, err := svc.Upload(drive.UploadInput{User: &model.User{ID: "u1"}, Body: []byte("x")})
	assert.ErrorIs(t, err, stubError)
}

func TestUpload_StoragePutError(t *testing.T) {
	storage := brokenStorage(t)
	idGen, _ := id.NewGenerator("aidx")
	svc := drive.NewService(testutil.NewMockDriveFileRepository(), testutil.NewMockDriveFolderRepository(), storage, idGen)
	_, err := svc.Upload(drive.UploadInput{User: &model.User{ID: "u1"}, Body: []byte("x")})
	assert.Error(t, err)
}

// errReader implements io.Reader and always returns an error.
type errReader struct{}

func (errReader) Read(_ []byte) (int, error) { return 0, stubError }

func TestUpload_AccessKeyGenError(t *testing.T) {
	restore := drive.SetRandReaderForTest(errReader{})
	defer restore()

	svc, _, _ := newSvc(t)
	_, err := svc.Upload(drive.UploadInput{User: &model.User{ID: "u1"}, Body: []byte("x")})
	assert.Error(t, err)
}

// --- Show / FindByHash ---

func TestShow_NilUser(t *testing.T) {
	svc, _, _ := newSvc(t)
	_, err := svc.Show(nil, "x")
	assert.Error(t, err)
}

func TestShow_NotFound(t *testing.T) {
	svc, _, _ := newSvc(t)
	_, err := svc.Show(&model.User{ID: "u1"}, "ghost")
	require.ErrorIs(t, err, drive.ErrFileNotFound)
}

func TestShow_OtherUserDenied(t *testing.T) {
	svc, fileRepo, _ := newSvc(t)
	other := "other"
	fileRepo.Files["f1"] = &model.DriveFile{ID: "f1", UserID: &other}
	_, err := svc.Show(&model.User{ID: "u1"}, "f1")
	require.ErrorIs(t, err, drive.ErrAccessDenied)
}

func TestFindByHash_NilUser(t *testing.T) {
	svc, _, _ := newSvc(t)
	_, err := svc.FindByHash(nil, "x")
	assert.Error(t, err)
}

func TestFindByHash_NotFound(t *testing.T) {
	svc, _, _ := newSvc(t)
	_, err := svc.FindByHash(&model.User{ID: "u1"}, "ghost")
	require.ErrorIs(t, err, drive.ErrFileNotFound)
}

func TestFindByHash_Found(t *testing.T) {
	svc, fileRepo, _ := newSvc(t)
	uid := "u1"
	fileRepo.Files["f1"] = &model.DriveFile{ID: "f1", UserID: &uid, MD5: "abc"}
	got, err := svc.FindByHash(&model.User{ID: "u1"}, "abc")
	require.NoError(t, err)
	assert.Equal(t, "f1", got.ID)
}

// --- Update ---

func TestUpdate_HappyPath(t *testing.T) {
	svc, fileRepo, _ := newSvc(t)
	uid := "u1"
	fileRepo.Files["f1"] = &model.DriveFile{ID: "f1", UserID: &uid, Name: "old"}

	newName := "new"
	cmt := "comment"
	cmtPtr := &cmt
	sensitive := true
	got, err := svc.Update(&model.User{ID: "u1"}, "f1", drive.UpdateInput{
		Name:        &newName,
		Comment:     &cmtPtr,
		IsSensitive: &sensitive,
	})
	require.NoError(t, err)
	assert.Equal(t, "new", got.Name)
}

func TestUpdate_NotFound(t *testing.T) {
	svc, _, _ := newSvc(t)
	_, err := svc.Update(&model.User{ID: "u1"}, "ghost", drive.UpdateInput{})
	require.ErrorIs(t, err, drive.ErrFileNotFound)
}

func TestUpdate_FolderNotFound(t *testing.T) {
	svc, fileRepo, _ := newSvc(t)
	uid := "u1"
	fileRepo.Files["f1"] = &model.DriveFile{ID: "f1", UserID: &uid}
	folderID := "ghost"
	folderPtr := &folderID
	_, err := svc.Update(&model.User{ID: "u1"}, "f1", drive.UpdateInput{FolderID: &folderPtr})
	require.ErrorIs(t, err, drive.ErrFolderNotFound)
}

func TestUpdate_FolderAccessDenied(t *testing.T) {
	svc, fileRepo, folderRepo := newSvc(t)
	uid := "u1"
	other := "other"
	fileRepo.Files["f1"] = &model.DriveFile{ID: "f1", UserID: &uid}
	folderRepo.Folders["fid"] = &model.DriveFolder{ID: "fid", UserID: &other}

	folderID := "fid"
	folderPtr := &folderID
	_, err := svc.Update(&model.User{ID: "u1"}, "f1", drive.UpdateInput{FolderID: &folderPtr})
	require.ErrorIs(t, err, drive.ErrAccessDenied)
}

func TestUpdate_FolderNullable(t *testing.T) {
	svc, fileRepo, _ := newSvc(t)
	uid := "u1"
	folderID := "x"
	fileRepo.Files["f1"] = &model.DriveFile{ID: "f1", UserID: &uid, FolderID: &folderID}

	var nilFolder *string
	_, err := svc.Update(&model.User{ID: "u1"}, "f1", drive.UpdateInput{FolderID: &nilFolder})
	require.NoError(t, err)
}

// failingUpdateFileRepo causes Update to fail.
type failingUpdateFileRepo struct {
	*testutil.MockDriveFileRepository
}

func (f *failingUpdateFileRepo) Update(_ string, _ map[string]any) error {
	return stubError
}

func TestUpdate_RepoUpdateError(t *testing.T) {
	mock := testutil.NewMockDriveFileRepository()
	uid := "u1"
	mock.Files["f1"] = &model.DriveFile{ID: "f1", UserID: &uid}
	repo := &failingUpdateFileRepo{MockDriveFileRepository: mock}
	idGen, _ := id.NewGenerator("aidx")
	svc := drive.NewService(repo, testutil.NewMockDriveFolderRepository(), drive.NewLocalStorage(t.TempDir(), ""), idGen)
	newName := "x"
	_, err := svc.Update(&model.User{ID: "u1"}, "f1", drive.UpdateInput{Name: &newName})
	assert.ErrorIs(t, err, stubError)
}

// --- Delete ---

func TestDelete_NotFound(t *testing.T) {
	svc, _, _ := newSvc(t)
	err := svc.Delete(&model.User{ID: "u1"}, "ghost")
	require.ErrorIs(t, err, drive.ErrFileNotFound)
}

func TestDelete_HappyPath(t *testing.T) {
	svc, fileRepo, _ := newSvc(t)
	user := &model.User{ID: "u1"}
	created, err := svc.Upload(drive.UploadInput{User: user, Body: []byte("x"), Name: "x"})
	require.NoError(t, err)
	require.NoError(t, svc.Delete(user, created.ID))
	assert.Empty(t, fileRepo.Files)
}

func TestDelete_StorageError(t *testing.T) {
	storage := brokenStorage(t)
	mock := testutil.NewMockDriveFileRepository()
	uid := "u1"
	key := "abc"
	mock.Files["f1"] = &model.DriveFile{ID: "f1", UserID: &uid, AccessKey: &key}
	idGen, _ := id.NewGenerator("aidx")
	svc := drive.NewService(mock, testutil.NewMockDriveFolderRepository(), storage, idGen)
	err := svc.Delete(&model.User{ID: "u1"}, "f1")
	assert.Error(t, err)
}

// --- Folders ---

func TestCreateFolder_NilUser(t *testing.T) {
	svc, _, _ := newSvc(t)
	_, err := svc.CreateFolder(nil, "x", nil)
	assert.Error(t, err)
}

func TestCreateFolder_HappyPath(t *testing.T) {
	svc, _, folderRepo := newSvc(t)
	f, err := svc.CreateFolder(&model.User{ID: "u1"}, "Hello", nil)
	require.NoError(t, err)
	assert.Equal(t, "Hello", f.Name)
	assert.Len(t, folderRepo.Folders, 1)
}

func TestCreateFolder_ParentNotFound(t *testing.T) {
	svc, _, _ := newSvc(t)
	pid := "ghost"
	_, err := svc.CreateFolder(&model.User{ID: "u1"}, "x", &pid)
	require.ErrorIs(t, err, drive.ErrFolderNotFound)
}

func TestCreateFolder_ParentAccessDenied(t *testing.T) {
	svc, _, folderRepo := newSvc(t)
	other := "other"
	folderRepo.Folders["p"] = &model.DriveFolder{ID: "p", UserID: &other}
	pid := "p"
	_, err := svc.CreateFolder(&model.User{ID: "u1"}, "x", &pid)
	require.ErrorIs(t, err, drive.ErrAccessDenied)
}

// failingCreateFolderRepo causes Create to fail.
type failingCreateFolderRepo struct {
	*testutil.MockDriveFolderRepository
}

func (f *failingCreateFolderRepo) Create(_ *model.DriveFolder) error {
	return stubError
}

func TestCreateFolder_RepoError(t *testing.T) {
	mock := testutil.NewMockDriveFolderRepository()
	repo := &failingCreateFolderRepo{MockDriveFolderRepository: mock}
	idGen, _ := id.NewGenerator("aidx")
	svc := drive.NewService(testutil.NewMockDriveFileRepository(), repo, drive.NewLocalStorage(t.TempDir(), ""), idGen)
	_, err := svc.CreateFolder(&model.User{ID: "u1"}, "x", nil)
	assert.ErrorIs(t, err, stubError)
}

func TestShowFolder_NilUser(t *testing.T) {
	svc, _, _ := newSvc(t)
	_, err := svc.ShowFolder(nil, "x")
	assert.Error(t, err)
}

func TestShowFolder_NotFound(t *testing.T) {
	svc, _, _ := newSvc(t)
	_, err := svc.ShowFolder(&model.User{ID: "u1"}, "ghost")
	require.ErrorIs(t, err, drive.ErrFolderNotFound)
}

func TestShowFolder_AccessDenied(t *testing.T) {
	svc, _, folderRepo := newSvc(t)
	other := "other"
	folderRepo.Folders["p"] = &model.DriveFolder{ID: "p", UserID: &other}
	_, err := svc.ShowFolder(&model.User{ID: "u1"}, "p")
	require.ErrorIs(t, err, drive.ErrAccessDenied)
}

func TestUpdateFolder_HappyPath(t *testing.T) {
	svc, _, folderRepo := newSvc(t)
	uid := "u1"
	folderRepo.Folders["p"] = &model.DriveFolder{ID: "p", Name: "Old", UserID: &uid}

	newName := "New"
	got, err := svc.UpdateFolder(&model.User{ID: "u1"}, "p", drive.UpdateFolderInput{Name: &newName})
	require.NoError(t, err)
	assert.Equal(t, "New", got.Name)
}

func TestUpdateFolder_NotFound(t *testing.T) {
	svc, _, _ := newSvc(t)
	_, err := svc.UpdateFolder(&model.User{ID: "u1"}, "ghost", drive.UpdateFolderInput{})
	require.ErrorIs(t, err, drive.ErrFolderNotFound)
}

func TestUpdateFolder_ParentNotFound(t *testing.T) {
	svc, _, folderRepo := newSvc(t)
	uid := "u1"
	folderRepo.Folders["c"] = &model.DriveFolder{ID: "c", UserID: &uid}
	pid := "ghost"
	pidPtr := &pid
	_, err := svc.UpdateFolder(&model.User{ID: "u1"}, "c", drive.UpdateFolderInput{ParentID: &pidPtr})
	require.ErrorIs(t, err, drive.ErrFolderNotFound)
}

func TestUpdateFolder_ParentAccessDenied(t *testing.T) {
	svc, _, folderRepo := newSvc(t)
	uid := "u1"
	other := "other"
	folderRepo.Folders["c"] = &model.DriveFolder{ID: "c", UserID: &uid}
	folderRepo.Folders["p"] = &model.DriveFolder{ID: "p", UserID: &other}
	pid := "p"
	pidPtr := &pid
	_, err := svc.UpdateFolder(&model.User{ID: "u1"}, "c", drive.UpdateFolderInput{ParentID: &pidPtr})
	require.ErrorIs(t, err, drive.ErrAccessDenied)
}

func TestUpdateFolder_NullParent(t *testing.T) {
	svc, _, folderRepo := newSvc(t)
	uid := "u1"
	pid := "p"
	folderRepo.Folders["c"] = &model.DriveFolder{ID: "c", UserID: &uid, ParentID: &pid}

	var nilParent *string
	_, err := svc.UpdateFolder(&model.User{ID: "u1"}, "c", drive.UpdateFolderInput{ParentID: &nilParent})
	require.NoError(t, err)
}

type failingUpdateFolderRepo struct {
	*testutil.MockDriveFolderRepository
}

func (f *failingUpdateFolderRepo) Update(_ string, _ map[string]any) error {
	return stubError
}

func TestUpdateFolder_RepoError(t *testing.T) {
	mock := testutil.NewMockDriveFolderRepository()
	uid := "u1"
	mock.Folders["c"] = &model.DriveFolder{ID: "c", UserID: &uid}
	repo := &failingUpdateFolderRepo{MockDriveFolderRepository: mock}
	idGen, _ := id.NewGenerator("aidx")
	svc := drive.NewService(testutil.NewMockDriveFileRepository(), repo, drive.NewLocalStorage(t.TempDir(), ""), idGen)
	newName := "x"
	_, err := svc.UpdateFolder(&model.User{ID: "u1"}, "c", drive.UpdateFolderInput{Name: &newName})
	assert.ErrorIs(t, err, stubError)
}

func TestDeleteFolder_NotFound(t *testing.T) {
	svc, _, _ := newSvc(t)
	err := svc.DeleteFolder(&model.User{ID: "u1"}, "ghost")
	require.ErrorIs(t, err, drive.ErrFolderNotFound)
}

func TestDeleteFolder_NotEmpty(t *testing.T) {
	svc, fileRepo, folderRepo := newSvc(t)
	uid := "u1"
	folderRepo.Folders["p"] = &model.DriveFolder{ID: "p", UserID: &uid}
	pid := "p"
	fileRepo.Files["f1"] = &model.DriveFile{ID: "f1", UserID: &uid, FolderID: &pid}
	err := svc.DeleteFolder(&model.User{ID: "u1"}, "p")
	require.ErrorIs(t, err, drive.ErrFolderNotEmpty)
}

func TestDeleteFolder_HappyPath(t *testing.T) {
	svc, _, folderRepo := newSvc(t)
	uid := "u1"
	folderRepo.Folders["p"] = &model.DriveFolder{ID: "p", UserID: &uid}
	require.NoError(t, svc.DeleteFolder(&model.User{ID: "u1"}, "p"))
	assert.Empty(t, folderRepo.Folders)
}

type failingHasChildrenRepo struct {
	*testutil.MockDriveFolderRepository
}

func (f *failingHasChildrenRepo) HasChildren(_ string) (bool, error) {
	return false, stubError
}

func TestDeleteFolder_HasChildrenError(t *testing.T) {
	mock := testutil.NewMockDriveFolderRepository()
	uid := "u1"
	mock.Folders["p"] = &model.DriveFolder{ID: "p", UserID: &uid}
	repo := &failingHasChildrenRepo{MockDriveFolderRepository: mock}
	idGen, _ := id.NewGenerator("aidx")
	svc := drive.NewService(testutil.NewMockDriveFileRepository(), repo, drive.NewLocalStorage(t.TempDir(), ""), idGen)
	err := svc.DeleteFolder(&model.User{ID: "u1"}, "p")
	assert.ErrorIs(t, err, stubError)
}
