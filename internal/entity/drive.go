package entity

import (
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
)

// DriveFileEntity is the drive file representation returned by API endpoints.
type DriveFileEntity struct {
	ID           string  `json:"id"`
	CreatedAt    string  `json:"createdAt"`
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	MD5          string  `json:"md5"`
	Size         int     `json:"size"`
	IsSensitive  bool    `json:"isSensitive"`
	Blurhash     *string `json:"blurhash"`
	Comment      *string `json:"comment"`
	URL          string  `json:"url"`
	ThumbnailURL *string `json:"thumbnailUrl"`
	FolderID     *string `json:"folderId"`
	UserID       *string `json:"userId"`
}

// PackDriveFile converts a model.DriveFile to a DriveFileEntity.
func PackDriveFile(f *model.DriveFile, idGen id.Generator) DriveFileEntity {
	createdAt := ""
	if t, err := idGen.ParseTime(f.ID); err == nil {
		createdAt = t.UTC().Format("2006-01-02T15:04:05.000Z")
	}
	return DriveFileEntity{
		ID:           f.ID,
		CreatedAt:    createdAt,
		Name:         f.Name,
		Type:         f.Type,
		MD5:          f.MD5,
		Size:         f.Size,
		IsSensitive:  f.IsSensitive,
		Blurhash:     f.Blurhash,
		Comment:      f.Comment,
		URL:          f.URL,
		ThumbnailURL: f.ThumbnailURL,
		FolderID:     f.FolderID,
		UserID:       f.UserID,
	}
}

// DriveFolderEntity is the drive folder representation returned by API endpoints.
type DriveFolderEntity struct {
	ID        string  `json:"id"`
	CreatedAt string  `json:"createdAt"`
	Name      string  `json:"name"`
	ParentID  *string `json:"parentId"`
}

// PackDriveFolder converts a model.DriveFolder to a DriveFolderEntity.
func PackDriveFolder(f *model.DriveFolder, idGen id.Generator) DriveFolderEntity {
	createdAt := ""
	if t, err := idGen.ParseTime(f.ID); err == nil {
		createdAt = t.UTC().Format("2006-01-02T15:04:05.000Z")
	}
	return DriveFolderEntity{
		ID:        f.ID,
		CreatedAt: createdAt,
		Name:      f.Name,
		ParentID:  f.ParentID,
	}
}
