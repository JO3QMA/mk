package entity

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
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

func TestPackDriveFile_WithPropertiesAndWebpublic(t *testing.T) {
	idGen := newTestIDGen(t)
	fileID := idGen.Generate(time.Now())
	uid := "u1"
	wpURL := "https://example.com/wp.webp"
	wpType := "image/webp"
	props := datatypes.JSON(`{"width":800,"height":600}`)

	f := &model.DriveFile{
		ID:            fileID,
		UserID:        &uid,
		Name:          "photo.jpg",
		Type:          "image/jpeg",
		Properties:    props,
		URL:           "https://example.com/files/x",
		WebpublicURL:  &wpURL,
		WebpublicType: &wpType,
	}

	out := PackDriveFile(f, idGen)
	assert.Equal(t, &wpURL, out.WebpublicURL)
	// typed DriveFileProperties に変換されているため、width/height が
	// ポインタ int で直接触れる。
	require.NotNil(t, out.Properties.Width)
	require.NotNil(t, out.Properties.Height)
	assert.Equal(t, 800, *out.Properties.Width)
	assert.Equal(t, 600, *out.Properties.Height)
}

func TestPackDriveFile_EmptyProperties(t *testing.T) {
	idGen := newTestIDGen(t)
	fileID := idGen.Generate(time.Now())

	f := &model.DriveFile{
		ID:   fileID,
		Name: "file.txt",
		URL:  "https://example.com/files/x",
	}
	out := PackDriveFile(f, idGen)
	// properties 空 → zero-valued DriveFileProperties (全フィールド nil)
	assert.Nil(t, out.Properties.Width)
	assert.Nil(t, out.Properties.Height)
	assert.Nil(t, out.Properties.Orientation)
	assert.Nil(t, out.Properties.AvgColor)
	assert.Nil(t, out.WebpublicURL)
	// Folder / User は pack 側でセットしないので nil のまま
	assert.Nil(t, out.Folder)
	assert.Nil(t, out.User)
}

func TestPackDriveFile_AllPropertyFields(t *testing.T) {
	idGen := newTestIDGen(t)
	fileID := idGen.Generate(time.Now())
	props, err := json.Marshal(map[string]any{
		"width":       1280,
		"height":      720,
		"orientation": 8,
		"avgColor":    "rgb(40,65,87)",
	})
	require.NoError(t, err)
	f := &model.DriveFile{
		ID:         fileID,
		Name:       "photo.jpg",
		URL:        "https://example.com/p.jpg",
		Properties: datatypes.JSON(props),
	}
	out := PackDriveFile(f, idGen)
	require.NotNil(t, out.Properties.Width)
	assert.Equal(t, 1280, *out.Properties.Width)
	require.NotNil(t, out.Properties.Height)
	assert.Equal(t, 720, *out.Properties.Height)
	require.NotNil(t, out.Properties.Orientation)
	assert.Equal(t, 8, *out.Properties.Orientation)
	require.NotNil(t, out.Properties.AvgColor)
	assert.Equal(t, "rgb(40,65,87)", *out.Properties.AvgColor)
}

func TestPackDriveFile_InvalidPropertiesJSON_Defaults(t *testing.T) {
	idGen := newTestIDGen(t)
	f := &model.DriveFile{
		ID:         idGen.Generate(time.Now()),
		Name:       "bad.jpg",
		URL:        "x",
		Properties: datatypes.JSON([]byte("not-json")),
	}
	out := PackDriveFile(f, idGen)
	// invalid JSON → 全フィールド nil にフォールバック
	assert.Nil(t, out.Properties.Width)
	assert.Nil(t, out.Properties.Height)
}

func TestPackDriveFile_WithFolderAndUser(t *testing.T) {
	idGen := newTestIDGen(t)
	fileID := idGen.Generate(time.Now())
	folderID := idGen.Generate(time.Now())
	userID := "u1"
	f := &model.DriveFile{ID: fileID, Name: "a.jpg", URL: "x", FolderID: &folderID, UserID: &userID}
	folder := &model.DriveFolder{ID: folderID, Name: "Pictures"}
	user := &model.User{ID: userID, Username: "alice"}

	out := PackDriveFile(f, idGen)
	// caller が pack して後付けするパターン
	packedFolder := PackDriveFolder(folder, idGen)
	out.Folder = &packedFolder
	lite := PackUserLite(user)
	out.User = &lite

	require.NotNil(t, out.Folder)
	assert.Equal(t, "Pictures", out.Folder.Name)
	require.NotNil(t, out.User)
	assert.Equal(t, "alice", out.User.Username)
}
