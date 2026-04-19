package entity

import (
	"encoding/json"

	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
)

// DriveFileProperties mirrors the `properties` sub-object in Misskey's
// packedDriveFileSchema. All fields are optional so omitempty produces the
// same JSON shape as TS (fields absent rather than null). Width/Height are
// in pixels, Orientation is the EXIF orientation code (1-8), and AvgColor
// is an "rgb(r,g,b)" string.
type DriveFileProperties struct {
	Width       *int    `json:"width,omitempty"`
	Height      *int    `json:"height,omitempty"`
	Orientation *int    `json:"orientation,omitempty"`
	AvgColor    *string `json:"avgColor,omitempty"`
}

// DriveFileEntity is the drive file representation returned by API endpoints.
type DriveFileEntity struct {
	ID           string              `json:"id"`
	CreatedAt    string              `json:"createdAt"`
	Name         string              `json:"name"`
	Type         string              `json:"type"`
	MD5          string              `json:"md5"`
	Size         int                 `json:"size"`
	IsSensitive  bool                `json:"isSensitive"`
	Blurhash     *string             `json:"blurhash"`
	Properties   DriveFileProperties `json:"properties"`
	Comment      *string             `json:"comment"`
	URL          string              `json:"url"`
	ThumbnailURL *string             `json:"thumbnailUrl"`
	WebpublicURL *string             `json:"webpublicUrl"`
	FolderID     *string             `json:"folderId"`
	UserID       *string             `json:"userId"`
	// Folder は optional (TS schema: folder?: DriveFolder | null)。caller が
	// pre-fetch してセットする。nil のときは omitempty で JSON から省略。
	Folder *DriveFolderEntity `json:"folder,omitempty"`
	// User は optional (TS schema: user?: UserLite | null)。同上。
	User *UserLite `json:"user,omitempty"`
}

// PackDriveFile converts a model.DriveFile to a DriveFileEntity. The Folder
// and User fields are left nil; callers that need the nested objects should
// fetch them via DriveFolderRepository / UserRepository and assign
// &PackDriveFolder(...) / &PackUserLite(...) to the returned entity.
func PackDriveFile(f *model.DriveFile, idGen id.Generator) DriveFileEntity {
	createdAt := ""
	if t, err := idGen.ParseTime(f.ID); err == nil {
		createdAt = t.UTC().Format("2006-01-02T15:04:05.000Z")
	}
	var props DriveFileProperties
	// model.Properties は datatypes.JSON (=[]byte)。パース失敗時は空struct
	// で返して UI を壊さない (CLAUDE.md「エラー処理は best-effort」方針)。
	if len(f.Properties) > 0 {
		_ = json.Unmarshal(f.Properties, &props)
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
		Properties:   props,
		Comment:      f.Comment,
		URL:          f.URL,
		ThumbnailURL: f.ThumbnailURL,
		WebpublicURL: f.WebpublicURL,
		FolderID:     f.FolderID,
		UserID:       f.UserID,
	}
}

// PackDriveFileWithRelations is PackDriveFile + optional folder/user
// embedding. The caller is responsible for fetching the related rows (so
// batch/cached access is possible). Pass nil for fields you do not want to
// expose — omitempty keeps the JSON output clean.
func PackDriveFileWithRelations(f *model.DriveFile, idGen id.Generator, folder *model.DriveFolder, user *model.User) DriveFileEntity {
	out := PackDriveFile(f, idGen)
	if folder != nil {
		packed := PackDriveFolder(folder, idGen)
		out.Folder = &packed
	}
	if user != nil {
		lite := PackUserLite(user)
		out.User = &lite
	}
	return out
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
