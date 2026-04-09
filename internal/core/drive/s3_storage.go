package drive

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3API abstracts the subset of S3 client operations used by S3Storage.
// テスト時はモックに差し替え可能。
type S3API interface {
	PutObject(ctx context.Context, input *s3.PutObjectInput, opts ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(ctx context.Context, input *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObject(ctx context.Context, input *s3.DeleteObjectInput, opts ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

// S3Storage implements Storage using an S3-compatible object storage backend.
type S3Storage struct {
	client        S3API
	bucket        string
	prefix        string // オブジェクトキーのプリフィックス (e.g. "files/")
	baseURL       string // 公開 URL のベース (e.g. "https://cdn.example.com")
	setPublicRead bool
}

// S3StorageConfig holds the configuration for creating an S3Storage.
type S3StorageConfig struct {
	Client        S3API
	Bucket        string
	Prefix        string
	BaseURL       string // 空の場合はエンドポイントから自動生成
	SetPublicRead bool
}

// NewS3Storage creates a new S3Storage instance.
func NewS3Storage(cfg S3StorageConfig) *S3Storage {
	return &S3Storage{
		client:        cfg.Client,
		bucket:        cfg.Bucket,
		prefix:        cfg.Prefix,
		baseURL:       strings.TrimRight(cfg.BaseURL, "/"),
		setPublicRead: cfg.SetPublicRead,
	}
}

// objectKey returns the full S3 object key for the given accessKey.
func (s *S3Storage) objectKey(accessKey string) string {
	return s.prefix + accessKey
}

// publicURL returns the public URL for the given accessKey.
func (s *S3Storage) publicURL(accessKey string) string {
	key := s.objectKey(accessKey)
	if s.baseURL != "" {
		return s.baseURL + "/" + key
	}
	// baseURL が未設定の場合は S3 パススタイル URL にフォールバック
	return fmt.Sprintf("https://s3.amazonaws.com/%s/%s", s.bucket, key)
}

// Put uploads the body to S3 and returns the public URL.
func (s *S3Storage) Put(accessKey string, body io.Reader) (string, error) {
	key := s.objectKey(accessKey)

	// MIME type を判定するため先頭を読む
	buf := make([]byte, 512)
	n, _ := io.ReadAtLeast(body, buf, 1)
	contentType := http.DetectContentType(buf[:n])
	// 読んだ分を body の先頭に戻す
	combined := io.MultiReader(strings.NewReader(string(buf[:n])), body)

	input := &s3.PutObjectInput{
		Bucket:       aws.String(s.bucket),
		Key:          aws.String(key),
		Body:         combined,
		ContentType:  aws.String(contentType),
		CacheControl: aws.String("max-age=31536000, immutable"),
	}
	if s.setPublicRead {
		input.ACL = types.ObjectCannedACLPublicRead
	}

	if _, err := s.client.PutObject(context.Background(), input); err != nil {
		return "", fmt.Errorf("s3 put %s: %w", key, err)
	}
	return s.publicURL(accessKey), nil
}

// Get retrieves an object from S3 and returns an io.ReadCloser.
func (s *S3Storage) Get(accessKey string) (io.ReadCloser, error) {
	key := s.objectKey(accessKey)
	output, err := s.client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, ErrObjectNotFound
	}
	return output.Body, nil
}

// Delete removes an object from S3. Missing objects are silently ignored.
func (s *S3Storage) Delete(accessKey string) error {
	key := s.objectKey(accessKey)
	_, err := s.client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("s3 delete %s: %w", key, err)
	}
	return nil
}
