package transfer

import (
	"fmt"
	"io"

	"github.com/shiroha-a/mk/internal/core/drive"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

// RepoBackedDriveReader implements DriveReader by pairing a DriveFileRepository
// (for metadata) with a drive.Storage (for the actual bytes). *drive.Service
// does not expose a public Get-by-id method, so the importer needs to wire
// these two itself.
type RepoBackedDriveReader struct {
	repo    repository.DriveFileRepository
	storage drive.Storage
}

// NewRepoBackedDriveReader constructs a RepoBackedDriveReader.
func NewRepoBackedDriveReader(repo repository.DriveFileRepository, storage drive.Storage) *RepoBackedDriveReader {
	return &RepoBackedDriveReader{repo: repo, storage: storage}
}

// Fetch looks up the DriveFile row and pulls its stored body into a single
// byte buffer. Only internally-stored files (StoredInternal=true) are
// supported; externally-hosted files would require HTTP fetch which is out
// of scope here.
func (r *RepoBackedDriveReader) Fetch(fileID string) (*model.DriveFile, []byte, error) {
	if r == nil || r.repo == nil {
		return nil, nil, fmt.Errorf("drive reader not initialized")
	}
	file, err := r.repo.FindByID(fileID)
	if err != nil {
		return nil, nil, fmt.Errorf("find drive file: %w", err)
	}
	if file.AccessKey == nil {
		return nil, nil, fmt.Errorf("drive file has no access key")
	}
	rc, err := r.storage.Get(*file.AccessKey)
	if err != nil {
		return nil, nil, fmt.Errorf("open drive object: %w", err)
	}
	defer func() { _ = rc.Close() }()
	body, err := io.ReadAll(rc)
	if err != nil {
		return nil, nil, fmt.Errorf("read drive object: %w", err)
	}
	return file, body, nil
}
