// Package drive provides DriveService for managing user-uploaded files.
package drive

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

// ErrObjectNotFound is returned when a stored object cannot be found.
var ErrObjectNotFound = errors.New("object not found")

// Storage abstracts the underlying object storage backend.
// Phase 2.5 では LocalStorage 実装のみ。S3/MinIO 等は Phase 3+ で追加する想定。
type Storage interface {
	// Put stores the body and returns the public URL where the object can be
	// fetched. accessKeyは保存先を一意に決定するための識別子で、Service層で生成する。
	Put(accessKey string, body io.Reader) (string, error)
	// Get returns a reader for an existing object.
	Get(accessKey string) (io.ReadCloser, error)
	// Delete removes an object. Missing objects do not produce an error.
	Delete(accessKey string) error
}

// LocalStorage stores objects on the local filesystem.
type LocalStorage struct {
	rootDir string
	baseURL string
}

// NewLocalStorage constructs a LocalStorage rooted at rootDir. baseURL is
// prepended to the access key when generating public URLs (typically
// "https://example.com/files").
func NewLocalStorage(rootDir, baseURL string) *LocalStorage {
	return &LocalStorage{rootDir: rootDir, baseURL: baseURL}
}

func (s *LocalStorage) path(accessKey string) string {
	return filepath.Join(s.rootDir, accessKey)
}

// Put writes body to disk and returns the public URL.
func (s *LocalStorage) Put(accessKey string, body io.Reader) (string, error) {
	if err := os.MkdirAll(s.rootDir, 0o755); err != nil {
		return "", err
	}
	f, err := os.Create(s.path(accessKey))
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, body); err != nil {
		return "", err
	}
	return s.baseURL + "/" + accessKey, nil
}

// Get returns a reader for the given object.
func (s *LocalStorage) Get(accessKey string) (io.ReadCloser, error) {
	f, err := os.Open(s.path(accessKey))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrObjectNotFound
		}
		return nil, err
	}
	return f, nil
}

// Delete removes the object from disk. Missing files are ignored.
func (s *LocalStorage) Delete(accessKey string) error {
	if err := os.Remove(s.path(accessKey)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
