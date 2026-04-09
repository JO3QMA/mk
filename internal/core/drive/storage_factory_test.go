package drive

import (
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
)

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

func TestNewStorageFromMeta_NilMeta(t *testing.T) {
	st := NewStorageFromMeta(nil, "/tmp/drive", "https://example.com/files")
	_, ok := st.(*LocalStorage)
	assert.True(t, ok, "nil meta should return LocalStorage")
}

func TestNewStorageFromMeta_UseObjectStorageFalse(t *testing.T) {
	meta := &model.Meta{UseObjectStorage: false}
	st := NewStorageFromMeta(meta, "/tmp/drive", "https://example.com/files")
	_, ok := st.(*LocalStorage)
	assert.True(t, ok)
}

func TestNewStorageFromMeta_NoBucket(t *testing.T) {
	meta := &model.Meta{UseObjectStorage: true}
	st := NewStorageFromMeta(meta, "/tmp/drive", "https://example.com/files")
	_, ok := st.(*LocalStorage)
	assert.True(t, ok, "empty bucket should fallback to LocalStorage")
}

func TestNewStorageFromMeta_EmptyBucket(t *testing.T) {
	meta := &model.Meta{
		UseObjectStorage:    true,
		ObjectStorageBucket: strPtr(""),
	}
	st := NewStorageFromMeta(meta, "/tmp/drive", "https://example.com/files")
	_, ok := st.(*LocalStorage)
	assert.True(t, ok)
}

func TestNewStorageFromMeta_S3(t *testing.T) {
	meta := &model.Meta{
		UseObjectStorage:              true,
		ObjectStorageBucket:           strPtr("my-bucket"),
		ObjectStoragePrefix:           strPtr("files/"),
		ObjectStorageBaseURL:          strPtr("https://cdn.example.com"),
		ObjectStorageEndpoint:         strPtr("s3.example.com"),
		ObjectStorageRegion:           strPtr("ap-northeast-1"),
		ObjectStorageAccessKey:        strPtr("AKID"),
		ObjectStorageSecretKey:        strPtr("SECRET"),
		ObjectStorageUseSSL:           true,
		ObjectStorageSetPublicRead:    true,
		ObjectStorageS3ForcePathStyle: true,
	}
	st := NewStorageFromMeta(meta, "/tmp/drive", "https://example.com/files")
	s3st, ok := st.(*S3Storage)
	assert.True(t, ok, "should return S3Storage")
	assert.Equal(t, "my-bucket", s3st.bucket)
	assert.Equal(t, "files/", s3st.prefix)
	assert.Equal(t, "https://cdn.example.com", s3st.baseURL)
	assert.True(t, s3st.setPublicRead)
}

func TestNewStorageFromMeta_S3_NilOptionalFields(t *testing.T) {
	meta := &model.Meta{
		UseObjectStorage:    true,
		ObjectStorageBucket: strPtr("bucket"),
	}
	st := NewStorageFromMeta(meta, "/tmp/drive", "https://example.com/files")
	_, ok := st.(*S3Storage)
	assert.True(t, ok)
}

// ---------------------------------------------------------------------------
// buildBaseURL tests
// ---------------------------------------------------------------------------

func TestBuildBaseURL_CustomBaseURL(t *testing.T) {
	meta := &model.Meta{
		ObjectStorageBaseURL: strPtr("https://cdn.example.com/files"),
	}
	assert.Equal(t, "https://cdn.example.com/files", buildBaseURL(meta))
}

func TestBuildBaseURL_AutoGenerate_HTTPS(t *testing.T) {
	meta := &model.Meta{
		ObjectStorageUseSSL:   true,
		ObjectStorageEndpoint: strPtr("s3.example.com"),
		ObjectStorageBucket:   strPtr("my-bucket"),
	}
	assert.Equal(t, "https://s3.example.com/my-bucket", buildBaseURL(meta))
}

func TestBuildBaseURL_AutoGenerate_HTTP(t *testing.T) {
	meta := &model.Meta{
		ObjectStorageUseSSL:   false,
		ObjectStorageEndpoint: strPtr("minio.local"),
		ObjectStorageBucket:   strPtr("bucket"),
	}
	assert.Equal(t, "http://minio.local/bucket", buildBaseURL(meta))
}

func TestBuildBaseURL_AutoGenerate_WithPort(t *testing.T) {
	meta := &model.Meta{
		ObjectStorageUseSSL:   false,
		ObjectStorageEndpoint: strPtr("minio.local"),
		ObjectStoragePort:     intPtr(9000),
		ObjectStorageBucket:   strPtr("bucket"),
	}
	assert.Equal(t, "http://minio.local:9000/bucket", buildBaseURL(meta))
}

func TestBuildBaseURL_AutoGenerate_NoEndpoint(t *testing.T) {
	meta := &model.Meta{
		ObjectStorageUseSSL: true,
		ObjectStorageBucket: strPtr("bucket"),
	}
	assert.Equal(t, "https://s3.amazonaws.com/bucket", buildBaseURL(meta))
}

func TestBuildBaseURL_EmptyBaseURL(t *testing.T) {
	meta := &model.Meta{
		ObjectStorageBaseURL: strPtr(""),
		ObjectStorageUseSSL:  true,
		ObjectStorageBucket:  strPtr("bucket"),
	}
	assert.Equal(t, "https://s3.amazonaws.com/bucket", buildBaseURL(meta))
}

func TestBuildBaseURL_ZeroPort(t *testing.T) {
	meta := &model.Meta{
		ObjectStorageEndpoint: strPtr("s3.example.com"),
		ObjectStoragePort:     intPtr(0),
		ObjectStorageBucket:   strPtr("bucket"),
		ObjectStorageUseSSL:   true,
	}
	// port=0 はポートなし扱い
	assert.Equal(t, "https://s3.example.com/bucket", buildBaseURL(meta))
}

// ---------------------------------------------------------------------------
// buildS3Client tests
// ---------------------------------------------------------------------------

func TestBuildS3Client_MinimalConfig(t *testing.T) {
	meta := &model.Meta{ObjectStorageUseSSL: true}
	client := buildS3Client(meta)
	assert.NotNil(t, client)
}

func TestBuildS3Client_FullConfig(t *testing.T) {
	meta := &model.Meta{
		ObjectStorageEndpoint:         strPtr("s3.example.com"),
		ObjectStoragePort:             intPtr(9000),
		ObjectStorageRegion:           strPtr("eu-west-1"),
		ObjectStorageAccessKey:        strPtr("AKID"),
		ObjectStorageSecretKey:        strPtr("SECRET"),
		ObjectStorageUseSSL:           false,
		ObjectStorageS3ForcePathStyle: true,
	}
	client := buildS3Client(meta)
	assert.NotNil(t, client)
}

func TestBuildS3Client_NoCredentials(t *testing.T) {
	meta := &model.Meta{
		ObjectStorageUseSSL: true,
		ObjectStorageRegion: strPtr("us-west-2"),
	}
	client := buildS3Client(meta)
	assert.NotNil(t, client)
}

func TestBuildS3Client_EmptyEndpoint(t *testing.T) {
	meta := &model.Meta{
		ObjectStorageEndpoint: strPtr(""),
		ObjectStorageUseSSL:   true,
	}
	client := buildS3Client(meta)
	assert.NotNil(t, client)
}

func TestBuildS3Client_EmptyRegion(t *testing.T) {
	meta := &model.Meta{
		ObjectStorageRegion: strPtr(""),
		ObjectStorageUseSSL: true,
	}
	client := buildS3Client(meta)
	assert.NotNil(t, client)
}

func TestBuildS3Client_PartialCredentials(t *testing.T) {
	meta := &model.Meta{
		ObjectStorageAccessKey: strPtr("AKID"),
		// SecretKey は nil
		ObjectStorageUseSSL: true,
	}
	client := buildS3Client(meta)
	assert.NotNil(t, client)
}
