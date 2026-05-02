package mediaproxy

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubLookup is a hand-rolled DriveFileLookup for the M1 test path.
type stubLookup struct {
	primary    string
	thumbKey   string
	webpubKey  string
	notFoundOn string // returns ErrNotFound when accessKey == notFoundOn
}

func (s stubLookup) FindByAccessKey(accessKey string) (DriveFileVariants, error) {
	if accessKey == s.notFoundOn {
		return DriveFileVariants{}, errors.New("not found")
	}
	primary := s.primary
	thumb := s.thumbKey
	webpub := s.webpubKey
	v := DriveFileVariants{}
	if primary != "" {
		v.AccessKey = &primary
	}
	if thumb != "" {
		v.ThumbnailAccessKey = &thumb
	}
	if webpub != "" {
		v.WebpublicAccessKey = &webpub
	}
	return v, nil
}

// TestResolveLocal_SwapsToThumbnail exercises M1: a local /files/<primary>
// request with ?preview should resolve to the thumbnail access key when one
// exists, skipping the proxy-side resize.
func TestResolveLocal_SwapsToThumbnail(t *testing.T) {
	store := newStubDriveStorage()
	store.put("primary-key", makePNG())
	store.put("thumb-key", makePNG())

	s := testService(map[string]bool{"https://example.com/files/primary-key": true})
	s.driveStorage = store
	s.SetDriveLookup(stubLookup{primary: "primary-key", thumbKey: "thumb-key"})

	res, err := s.Fetch(context.Background(), "https://example.com/files/primary-key", ModePreview, FormatWebP)
	require.NoError(t, err)
	defer res.Body.Close()

	// thumbnail key が読まれたら store にアクセスログが残る
	assert.Contains(t, store.reads, "thumb-key", "thumbnail variant should have been served")
	assert.NotContains(t, store.reads, "primary-key")
}

// TestResolveLocal_FallsBackWhenLookupMisses ensures missing lookup leaves the
// behavior identical to before M1 (serves the primary access key).
func TestResolveLocal_FallsBackWhenLookupMisses(t *testing.T) {
	store := newStubDriveStorage()
	store.put("primary-key", makePNG())

	s := testService(map[string]bool{"https://example.com/files/primary-key": true})
	s.driveStorage = store
	s.SetDriveLookup(stubLookup{notFoundOn: "primary-key"})

	res, err := s.Fetch(context.Background(), "https://example.com/files/primary-key", ModePreview, FormatWebP)
	require.NoError(t, err)
	defer res.Body.Close()

	assert.Contains(t, store.reads, "primary-key")
}

// TestResolveLocal_StaticPrefersWebpublic : Static mode prefers webpublic over
// thumbnail (mid-size variant > small thumbnail).
func TestResolveLocal_StaticPrefersWebpublic(t *testing.T) {
	store := newStubDriveStorage()
	store.put("primary-key", makePNG())
	store.put("thumb-key", makePNG())
	store.put("webpub-key", makePNG())

	s := testService(map[string]bool{"https://example.com/files/primary-key": true})
	s.driveStorage = store
	s.SetDriveLookup(stubLookup{primary: "primary-key", thumbKey: "thumb-key", webpubKey: "webpub-key"})

	_, err := s.Fetch(context.Background(), "https://example.com/files/primary-key", ModeStatic, FormatWebP)
	require.NoError(t, err)
	assert.Contains(t, store.reads, "webpub-key")
}

// TestResolveLocal_StaticFallsBackToThumbnail : when webpublic is missing,
// Static falls back to thumbnail.
func TestResolveLocal_StaticFallsBackToThumbnail(t *testing.T) {
	store := newStubDriveStorage()
	store.put("primary-key", makePNG())
	store.put("thumb-key", makePNG())

	s := testService(map[string]bool{"https://example.com/files/primary-key": true})
	s.driveStorage = store
	s.SetDriveLookup(stubLookup{primary: "primary-key", thumbKey: "thumb-key"})

	_, err := s.Fetch(context.Background(), "https://example.com/files/primary-key", ModeStatic, FormatWebP)
	require.NoError(t, err)
	assert.Contains(t, store.reads, "thumb-key")
}

// TestResolveLocal_RequestingVariantKeyDirectly : if the URL points at the
// thumbnail access key directly (not the primary), we MUST NOT chain another
// swap — serve as-is.
func TestResolveLocal_RequestingVariantKeyDirectly(t *testing.T) {
	store := newStubDriveStorage()
	store.put("thumb-key", makePNG())

	s := testService(map[string]bool{"https://example.com/files/thumb-key": true})
	s.driveStorage = store
	s.SetDriveLookup(stubLookup{primary: "primary-key", thumbKey: "thumb-key"})

	_, err := s.Fetch(context.Background(), "https://example.com/files/thumb-key", ModePreview, FormatWebP)
	require.NoError(t, err)
	assert.Equal(t, []string{"thumb-key"}, store.reads)
}

// TestResolveLocal_PreviewWithOnlyWebpublic : Preview falls back to webpublic
// when no thumbnail is available.
func TestResolveLocal_PreviewWithOnlyWebpublic(t *testing.T) {
	store := newStubDriveStorage()
	store.put("primary-key", makePNG())
	store.put("webpub-key", makePNG())

	s := testService(map[string]bool{"https://example.com/files/primary-key": true})
	s.driveStorage = store
	s.SetDriveLookup(stubLookup{primary: "primary-key", webpubKey: "webpub-key"})

	_, err := s.Fetch(context.Background(), "https://example.com/files/primary-key", ModePreview, FormatWebP)
	require.NoError(t, err)
	assert.Contains(t, store.reads, "webpub-key")
}

// TestResolveLocal_DefaultModeNoSwap : default mode (no resize) should NOT swap
// to thumbnail. The user explicitly asked for the original.
func TestResolveLocal_DefaultModeNoSwap(t *testing.T) {
	store := newStubDriveStorage()
	store.put("primary-key", makePNG())
	store.put("thumb-key", makePNG())

	s := testService(map[string]bool{"https://example.com/files/primary-key": true})
	s.driveStorage = store
	s.SetDriveLookup(stubLookup{primary: "primary-key", thumbKey: "thumb-key"})

	_, err := s.Fetch(context.Background(), "https://example.com/files/primary-key", ModeDefault, FormatWebP)
	require.NoError(t, err)
	assert.Contains(t, store.reads, "primary-key")
	assert.NotContains(t, store.reads, "thumb-key")
}

// stubDriveStorage records read access to support the swap-detection assertion.
type stubDriveStorage struct {
	objects map[string][]byte
	reads   []string
}

func newStubDriveStorage() *stubDriveStorage {
	return &stubDriveStorage{objects: map[string][]byte{}}
}

func (s *stubDriveStorage) put(key string, body []byte) { s.objects[key] = body }

func (s *stubDriveStorage) Get(key string) (io.ReadCloser, error) {
	s.reads = append(s.reads, key)
	body, ok := s.objects[key]
	if !ok {
		return nil, errors.New("object not found")
	}
	return io.NopCloser(strings.NewReader(string(body))), nil
}

// Put / Delete satisfy coredrive.Storage; not exercised here.
func (s *stubDriveStorage) Put(_ string, _ io.Reader) (string, error) { return "", nil }
func (s *stubDriveStorage) Delete(_ string) error                     { return nil }
