package drive

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"time"

	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

// Errors returned by Service.
var (
	// ErrFileNotFound is returned when the target file does not exist.
	ErrFileNotFound = errors.New("drive file not found")
	// ErrFolderNotFound is returned when the target folder does not exist.
	ErrFolderNotFound = errors.New("drive folder not found")
	// ErrAccessDenied is returned when a file/folder is owned by another user.
	ErrAccessDenied = errors.New("access denied")
	// ErrFolderNotEmpty is returned when attempting to delete a folder that
	// still has children.
	ErrFolderNotEmpty = errors.New("folder is not empty")
)

// StreamingPublisher receives drive file life-cycle events so that
// WebSocket subscribers (the "drive" channel) can be pushed in real time.
// パッケージ間の循環依存を避けるため interface で受け取る (実装は internal/
// stream)。eventType は "fileCreated" / "fileUpdated" / "fileDeleted"。
type StreamingPublisher interface {
	PublishDriveEvent(userID, eventType string, file *model.DriveFile)
}

// Service manages drive files and folders.
type Service struct {
	fileRepo   repository.DriveFileRepository
	folderRepo repository.DriveFolderRepository
	storage    Storage
	idGen      id.Generator
	publisher  StreamingPublisher
}

// NewService constructs a DriveService.
func NewService(
	fileRepo repository.DriveFileRepository,
	folderRepo repository.DriveFolderRepository,
	storage Storage,
	idGen id.Generator,
) *Service {
	return &Service{
		fileRepo:   fileRepo,
		folderRepo: folderRepo,
		storage:    storage,
		idGen:      idGen,
	}
}

// SetStreamingPublisher attaches a StreamingPublisher invoked best-effort
// after Upload / Update / Delete succeed.
func (s *Service) SetStreamingPublisher(p StreamingPublisher) {
	s.publisher = p
}

// publishEvent is a tiny best-effort wrapper around publisher.PublishDriveEvent.
func (s *Service) publishEvent(userID, eventType string, f *model.DriveFile) {
	if s.publisher == nil || userID == "" {
		return
	}
	s.publisher.PublishDriveEvent(userID, eventType, f)
}

// UploadInput is the parameter set for Service.Upload.
type UploadInput struct {
	User        *model.User
	Body        []byte
	Name        string
	Comment     *string
	FolderID    *string
	IsSensitive bool
	Force       bool // if true, do not deduplicate by md5 hash
}

// Upload writes a file to storage and creates a drive_file row. もし同じmd5の
// ファイルが既に存在し Force=false なら、既存レコードを返す (deduplication)。
func (s *Service) Upload(in UploadInput) (*model.DriveFile, error) {
	if in.User == nil {
		return nil, errors.New("user is required")
	}

	// in.Body は []byte なので bytes.Reader 経由のAnalyseFileは失敗しない。
	// AnalyseFile自体は io.Reader を取るがエラー経路はここでは到達しない。
	info, _ := AnalyseFile(bytes.NewReader(in.Body))

	if !in.Force {
		if existing, err := s.fileRepo.FindByMD5(in.User.ID, info.MD5); err == nil {
			return existing, nil
		}
	}

	// folderの所有権チェック
	if in.FolderID != nil {
		folder, err := s.folderRepo.FindByID(*in.FolderID)
		if err != nil {
			return nil, ErrFolderNotFound
		}
		if folder.UserID == nil || *folder.UserID != in.User.ID {
			return nil, ErrAccessDenied
		}
	}

	accessKey, err := newAccessKey()
	if err != nil {
		return nil, err
	}
	url, err := s.storage.Put(accessKey, bytes.NewReader(info.Body))
	if err != nil {
		return nil, err
	}

	now := time.Now()
	fileID := s.idGen.Generate(now)
	userID := in.User.ID
	f := &model.DriveFile{
		ID:             fileID,
		UserID:         &userID,
		UserHost:       in.User.Host,
		MD5:            info.MD5,
		Name:           in.Name,
		Type:           info.MimeType,
		Size:           info.Size,
		Comment:        in.Comment,
		StoredInternal: true,
		URL:            url,
		AccessKey:      &accessKey,
		FolderID:       in.FolderID,
		IsSensitive:    in.IsSensitive,
	}
	if err := s.fileRepo.Create(f); err != nil {
		// ロールバックとして storage を削除する
		_ = s.storage.Delete(accessKey)
		return nil, err
	}
	s.publishEvent(in.User.ID, "fileCreated", f)
	return f, nil
}

// Show returns the file with id, ensuring the requesting user owns it.
func (s *Service) Show(user *model.User, id string) (*model.DriveFile, error) {
	if user == nil {
		return nil, errors.New("user is required")
	}
	f, err := s.fileRepo.FindByID(id)
	if err != nil {
		return nil, ErrFileNotFound
	}
	if f.UserID == nil || *f.UserID != user.ID {
		return nil, ErrAccessDenied
	}
	return f, nil
}

// FindByHash returns the user's most recent file with the given md5 hash.
func (s *Service) FindByHash(user *model.User, md5 string) (*model.DriveFile, error) {
	if user == nil {
		return nil, errors.New("user is required")
	}
	f, err := s.fileRepo.FindByMD5(user.ID, md5)
	if err != nil {
		return nil, ErrFileNotFound
	}
	return f, nil
}

// UpdateInput holds the editable fields of a drive file.
type UpdateInput struct {
	Name        *string
	Comment     **string
	FolderID    **string
	IsSensitive *bool
}

// Update applies the non-nil fields to a drive file owned by the user.
func (s *Service) Update(user *model.User, id string, in UpdateInput) (*model.DriveFile, error) {
	f, err := s.Show(user, id)
	if err != nil {
		return nil, err
	}
	fields := map[string]any{}
	if in.Name != nil {
		fields["name"] = *in.Name
	}
	if in.Comment != nil {
		fields["comment"] = *in.Comment
	}
	if in.FolderID != nil {
		// folderの所有権チェック
		if *in.FolderID != nil {
			folder, err := s.folderRepo.FindByID(**in.FolderID)
			if err != nil {
				return nil, ErrFolderNotFound
			}
			if folder.UserID == nil || *folder.UserID != user.ID {
				return nil, ErrAccessDenied
			}
		}
		fields["folderId"] = *in.FolderID
	}
	if in.IsSensitive != nil {
		fields["isSensitive"] = *in.IsSensitive
	}
	if err := s.fileRepo.Update(f.ID, fields); err != nil {
		return nil, err
	}
	updated, err := s.fileRepo.FindByID(f.ID)
	if err != nil {
		return nil, err
	}
	s.publishEvent(user.ID, "fileUpdated", updated)
	return updated, nil
}

// Delete removes a file from storage and the database.
func (s *Service) Delete(user *model.User, id string) error {
	f, err := s.Show(user, id)
	if err != nil {
		return err
	}
	if f.AccessKey != nil {
		if err := s.storage.Delete(*f.AccessKey); err != nil {
			return err
		}
	}
	if err := s.fileRepo.Delete(f); err != nil {
		return err
	}
	s.publishEvent(user.ID, "fileDeleted", f)
	return nil
}

// CreateFolder creates a new drive folder owned by user.
func (s *Service) CreateFolder(user *model.User, name string, parentID *string) (*model.DriveFolder, error) {
	if user == nil {
		return nil, errors.New("user is required")
	}
	if parentID != nil {
		parent, err := s.folderRepo.FindByID(*parentID)
		if err != nil {
			return nil, ErrFolderNotFound
		}
		if parent.UserID == nil || *parent.UserID != user.ID {
			return nil, ErrAccessDenied
		}
	}
	userID := user.ID
	f := &model.DriveFolder{
		ID:       s.idGen.Generate(time.Now()),
		Name:     name,
		UserID:   &userID,
		ParentID: parentID,
	}
	if err := s.folderRepo.Create(f); err != nil {
		return nil, err
	}
	return f, nil
}

// ShowFolder returns the folder with id, ensuring the user owns it.
func (s *Service) ShowFolder(user *model.User, id string) (*model.DriveFolder, error) {
	if user == nil {
		return nil, errors.New("user is required")
	}
	f, err := s.folderRepo.FindByID(id)
	if err != nil {
		return nil, ErrFolderNotFound
	}
	if f.UserID == nil || *f.UserID != user.ID {
		return nil, ErrAccessDenied
	}
	return f, nil
}

// UpdateFolderInput holds the editable fields of a drive folder.
type UpdateFolderInput struct {
	Name     *string
	ParentID **string
}

// UpdateFolder applies the non-nil fields to a folder owned by user.
func (s *Service) UpdateFolder(user *model.User, id string, in UpdateFolderInput) (*model.DriveFolder, error) {
	f, err := s.ShowFolder(user, id)
	if err != nil {
		return nil, err
	}
	fields := map[string]any{}
	if in.Name != nil {
		fields["name"] = *in.Name
	}
	if in.ParentID != nil {
		if *in.ParentID != nil {
			parent, err := s.folderRepo.FindByID(**in.ParentID)
			if err != nil {
				return nil, ErrFolderNotFound
			}
			if parent.UserID == nil || *parent.UserID != user.ID {
				return nil, ErrAccessDenied
			}
		}
		fields["parentId"] = *in.ParentID
	}
	if err := s.folderRepo.Update(f.ID, fields); err != nil {
		return nil, err
	}
	return s.folderRepo.FindByID(f.ID)
}

// DeleteFolder removes an empty folder owned by user.
func (s *Service) DeleteFolder(user *model.User, id string) error {
	f, err := s.ShowFolder(user, id)
	if err != nil {
		return err
	}
	hasChildren, err := s.folderRepo.HasChildren(f.ID)
	if err != nil {
		return err
	}
	if hasChildren {
		return ErrFolderNotEmpty
	}
	return s.folderRepo.Delete(f)
}

// randReader is the source of randomness for newAccessKey. Tests override
// this to exercise the error path.
var randReader io.Reader = rand.Reader

// newAccessKey returns a random 32-character hex string used as the storage
// access key (and the URL path component for LocalStorage).
func newAccessKey() (string, error) {
	b := make([]byte, 16)
	if _, err := io.ReadFull(randReader, b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
