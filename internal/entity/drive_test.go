package entity

import (
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackDriveFile_Basic(t *testing.T) {
	idGen := newTestIDGen(t)
	fileID := idGen.Generate(time.Now())
	uid := "u1"
	cmt := "comment"
	bh := "blurhash"
	thumb := "https://example.com/t.jpg"
	folderID := "fid"

	f := &model.DriveFile{
		ID:           fileID,
		UserID:       &uid,
		MD5:          "abc",
		Name:         "hello.txt",
		Type:         "text/plain",
		Size:         5,
		IsSensitive:  true,
		Blurhash:     &bh,
		Comment:      &cmt,
		URL:          "https://example.com/files/x",
		ThumbnailURL: &thumb,
		FolderID:     &folderID,
	}

	out := PackDriveFile(f, idGen)
	assert.Equal(t, fileID, out.ID)
	assert.NotEmpty(t, out.CreatedAt)
	assert.Equal(t, "hello.txt", out.Name)
	assert.Equal(t, "abc", out.MD5)
	assert.Equal(t, 5, out.Size)
	assert.True(t, out.IsSensitive)
	assert.Equal(t, &bh, out.Blurhash)
	assert.Equal(t, &cmt, out.Comment)
	assert.Equal(t, &thumb, out.ThumbnailURL)
	assert.Equal(t, &folderID, out.FolderID)
}

func TestPackDriveFile_InvalidIDLeavesCreatedAtEmpty(t *testing.T) {
	idGen := newTestIDGen(t)
	f := &model.DriveFile{ID: "not-a-valid-id"}
	out := PackDriveFile(f, idGen)
	assert.Empty(t, out.CreatedAt)
}

func TestPackDriveFolder_Basic(t *testing.T) {
	idGen := newTestIDGen(t)
	folderID := idGen.Generate(time.Now())
	parentID := "parent"

	f := &model.DriveFolder{
		ID:       folderID,
		Name:     "Test Folder",
		ParentID: &parentID,
	}
	out := PackDriveFolder(f, idGen)
	assert.Equal(t, folderID, out.ID)
	assert.NotEmpty(t, out.CreatedAt)
	assert.Equal(t, "Test Folder", out.Name)
	assert.Equal(t, &parentID, out.ParentID)
}

func TestPackDriveFolder_InvalidIDLeavesCreatedAtEmpty(t *testing.T) {
	idGen := newTestIDGen(t)
	f := &model.DriveFolder{ID: "not-a-valid-id", Name: "x"}
	out := PackDriveFolder(f, idGen)
	assert.Empty(t, out.CreatedAt)
}

// Smoke test ensuring require import is wired (silences "imported and not used")
func TestPackDriveFile_RequireImport(t *testing.T) {
	require.NotNil(t, &model.DriveFile{})
}
