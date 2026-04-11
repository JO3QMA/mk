package emojiimport_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/core/drive"
	"github.com/shiroha-a/mk/internal/core/emojiimport"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- helpers ---

func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

type zipEntry struct {
	name string
	body []byte
}

func buildZip(t *testing.T, entries []zipEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		w, err := zw.Create(e.name)
		require.NoError(t, err)
		_, err = w.Write(e.body)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

// metaJSON returns a Misskey-compatible meta.json body for the given records.
func metaJSON(t *testing.T, records []map[string]any) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"metaVersion": 2,
		"host":        nil,
		"exportedAt":  time.Now().UTC().Format(time.RFC3339),
		"emojis":      records,
	})
	require.NoError(t, err)
	return b
}

// fakeDriveReader returns a fixed body (or error) for any fileID.
type fakeDriveReader struct {
	body []byte
	err  error
}

func (f *fakeDriveReader) Fetch(_ string) (*model.DriveFile, []byte, error) {
	if f.err != nil {
		return nil, nil, f.err
	}
	return &model.DriveFile{}, f.body, nil
}

func newUploader(t *testing.T) *drive.Service {
	t.Helper()
	fileRepo := testutil.NewMockDriveFileRepository()
	folderRepo := testutil.NewMockDriveFolderRepository()
	folderRepo.FilesRef = fileRepo
	storage := drive.NewLocalStorage(t.TempDir(), "https://example.com/files")
	idGen, _ := id.NewGenerator("aidx")
	return drive.NewService(fileRepo, folderRepo, storage, idGen)
}

func newDeps(t *testing.T, body []byte) (emojiimport.Deps, *testutil.MockUserRepository, *testutil.MockEmojiRepository, *drive.Service) {
	t.Helper()
	userRepo := testutil.NewMockUserRepository()
	_ = userRepo.Create(&model.User{ID: "admin"})
	emojiRepo := testutil.NewMockEmojiRepository()
	uploader := newUploader(t)
	reader := &fakeDriveReader{body: body}
	idGen, _ := id.NewGenerator("aidx")
	return emojiimport.Deps{
		UserRepo:  userRepo,
		EmojiRepo: emojiRepo,
		Drive:     reader,
		Uploader:  uploader,
		IDGen:     idGen,
	}, userRepo, emojiRepo, uploader
}

// --- Run ---

func TestRun_HappyPath_MultipleEmojis(t *testing.T) {
	img := pngBytes(t)
	meta := metaJSON(t, []map[string]any{
		{
			"fileName":   "smile.png",
			"downloaded": true,
			"emoji": map[string]any{
				"name":        "smile",
				"category":    "faces",
				"aliases":     []string{"happy", "joy"},
				"license":     "CC0",
				"isSensitive": false,
				"localOnly":   false,
			},
		},
		{
			"fileName":   "wave.png",
			"downloaded": true,
			"emoji": map[string]any{
				"name":    "wave",
				"aliases": []string{},
			},
		},
	})
	body := buildZip(t, []zipEntry{
		{"meta.json", meta},
		{"smile.png", img},
		{"wave.png", img},
	})
	deps, _, repo, _ := newDeps(t, body)
	imp := emojiimport.NewImporter(deps)

	res, err := imp.Run(context.Background(), "admin", "f1")
	require.NoError(t, err)
	assert.Equal(t, 2, res.Total)
	assert.Equal(t, 2, res.Imported)
	assert.Equal(t, 0, res.Skipped)

	smile, err := repo.FindByNameAndHost("smile", nil)
	require.NoError(t, err)
	assert.Equal(t, "CC0", *smile.License)
	require.NotNil(t, smile.Category)
	assert.Equal(t, "faces", *smile.Category)
	assert.Equal(t, []string{"happy", "joy"}, []string(smile.Aliases))

	wave, err := repo.FindByNameAndHost("wave", nil)
	require.NoError(t, err)
	assert.Nil(t, wave.Category)
}

func TestRun_DeletesExistingLocalEmoji(t *testing.T) {
	img := pngBytes(t)
	meta := metaJSON(t, []map[string]any{
		{"fileName": "smile.png", "downloaded": true, "emoji": map[string]any{"name": "smile"}},
	})
	body := buildZip(t, []zipEntry{{"meta.json", meta}, {"smile.png", img}})
	deps, _, repo, _ := newDeps(t, body)
	// 既存ローカル smile を置いて、import 後に ID が更新されていることを確認する。
	_ = repo.Create(&model.Emoji{ID: "old", Name: "smile"})

	imp := emojiimport.NewImporter(deps)
	_, err := imp.Run(context.Background(), "admin", "f1")
	require.NoError(t, err)

	found, err := repo.FindByNameAndHost("smile", nil)
	require.NoError(t, err)
	assert.NotEqual(t, "old", found.ID)
}

func TestRun_SkipsInvalidRecords(t *testing.T) {
	img := pngBytes(t)
	meta := metaJSON(t, []map[string]any{
		// not downloaded
		{"fileName": "a.png", "downloaded": false, "emoji": map[string]any{"name": "a"}},
		// invalid file name
		{"fileName": "bad name.png", "downloaded": true, "emoji": map[string]any{"name": "bad"}},
		// invalid emoji name
		{"fileName": "c.png", "downloaded": true, "emoji": map[string]any{"name": "bad!name"}},
		// missing zip entry
		{"fileName": "missing.png", "downloaded": true, "emoji": map[string]any{"name": "miss"}},
	})
	body := buildZip(t, []zipEntry{{"meta.json", meta}, {"a.png", img}, {"c.png", img}})
	deps, _, repo, _ := newDeps(t, body)
	imp := emojiimport.NewImporter(deps)

	res, err := imp.Run(context.Background(), "admin", "f1")
	require.NoError(t, err)
	assert.Equal(t, 4, res.Total)
	assert.Equal(t, 0, res.Imported)
	assert.Equal(t, 4, res.Skipped)
	assert.Empty(t, repo.Emojis)
}

func TestRun_MissingMeta(t *testing.T) {
	body := buildZip(t, []zipEntry{{"only.png", []byte("x")}})
	deps, _, _, _ := newDeps(t, body)
	imp := emojiimport.NewImporter(deps)
	_, err := imp.Run(context.Background(), "admin", "f1")
	assert.ErrorIs(t, err, emojiimport.ErrMissingMeta)
}

func TestRun_MalformedMeta(t *testing.T) {
	body := buildZip(t, []zipEntry{{"meta.json", []byte("not json")}})
	deps, _, _, _ := newDeps(t, body)
	imp := emojiimport.NewImporter(deps)
	_, err := imp.Run(context.Background(), "admin", "f1")
	assert.ErrorIs(t, err, emojiimport.ErrMalformedMeta)
}

func TestRun_InvalidZip(t *testing.T) {
	deps, _, _, _ := newDeps(t, []byte("not a zip"))
	imp := emojiimport.NewImporter(deps)
	_, err := imp.Run(context.Background(), "admin", "f1")
	assert.ErrorIs(t, err, emojiimport.ErrInvalidZip)
}

func TestRun_DriveFetchError(t *testing.T) {
	userRepo := testutil.NewMockUserRepository()
	_ = userRepo.Create(&model.User{ID: "admin"})
	idGen, _ := id.NewGenerator("aidx")
	deps := emojiimport.Deps{
		UserRepo:  userRepo,
		EmojiRepo: testutil.NewMockEmojiRepository(),
		Drive:     &fakeDriveReader{err: errors.New("boom")},
		Uploader:  newUploader(t),
		IDGen:     idGen,
	}
	imp := emojiimport.NewImporter(deps)
	_, err := imp.Run(context.Background(), "admin", "f1")
	assert.ErrorIs(t, err, emojiimport.ErrDriveFileNotFound)
}

func TestRun_UserNotFound(t *testing.T) {
	deps, _, _, _ := newDeps(t, []byte{})
	imp := emojiimport.NewImporter(deps)
	_, err := imp.Run(context.Background(), "ghost", "f1")
	assert.ErrorIs(t, err, emojiimport.ErrUserNotFound)
}

func TestRun_MissingDeps(t *testing.T) {
	imp := emojiimport.NewImporter(emojiimport.Deps{})
	_, err := imp.Run(context.Background(), "admin", "f1")
	assert.Error(t, err)
}

func TestRun_ContextCancelled(t *testing.T) {
	img := pngBytes(t)
	meta := metaJSON(t, []map[string]any{
		{"fileName": "smile.png", "downloaded": true, "emoji": map[string]any{"name": "smile"}},
	})
	body := buildZip(t, []zipEntry{{"meta.json", meta}, {"smile.png", img}})
	deps, _, _, _ := newDeps(t, body)
	imp := emojiimport.NewImporter(deps)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := imp.Run(ctx, "admin", "f1")
	assert.ErrorIs(t, err, context.Canceled)
}

func TestRun_UploaderError(t *testing.T) {
	img := pngBytes(t)
	meta := metaJSON(t, []map[string]any{
		{"fileName": "smile.png", "downloaded": true, "emoji": map[string]any{"name": "smile"}},
	})
	body := buildZip(t, []zipEntry{{"meta.json", meta}, {"smile.png", img}})
	// broken storage causes Upload to fail (storage Put fails).
	userRepo := testutil.NewMockUserRepository()
	_ = userRepo.Create(&model.User{ID: "admin"})
	emojiRepo := testutil.NewMockEmojiRepository()
	storage := drive.NewLocalStorage("/nonexistent/\x00invalid", "")
	fileRepo := testutil.NewMockDriveFileRepository()
	folderRepo := testutil.NewMockDriveFolderRepository()
	idGen, _ := id.NewGenerator("aidx")
	uploader := drive.NewService(fileRepo, folderRepo, storage, idGen)
	deps := emojiimport.Deps{
		UserRepo:  userRepo,
		EmojiRepo: emojiRepo,
		Drive:     &fakeDriveReader{body: body},
		Uploader:  uploader,
		IDGen:     idGen,
	}
	imp := emojiimport.NewImporter(deps)
	res, err := imp.Run(context.Background(), "admin", "f1")
	require.NoError(t, err)
	assert.Equal(t, 1, res.Skipped)
	assert.Equal(t, 0, res.Imported)
}

func TestSetNow(t *testing.T) {
	deps, _, _, _ := newDeps(t, []byte{})
	imp := emojiimport.NewImporter(deps)
	imp.SetNow(func() time.Time { return time.Unix(42, 0) })
	imp.SetNow(nil) // nil must be ignored
}
