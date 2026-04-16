package repository

import (
	"context"
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cleanupEmoji(t *testing.T, id string) {
	t.Helper()
	testDB.Exec(`DELETE FROM "emoji" WHERE id = ?`, id)
}

func TestEmojiRepository_FindByNameAndHost_Local(t *testing.T) {
	repo := NewEmojiRepository(testDB)

	e := &model.Emoji{
		ID:          "e_local",
		Name:        "smile",
		OriginalURL: "https://example.com/smile.png",
	}
	require.NoError(t, testDB.Create(e).Error)
	defer cleanupEmoji(t, e.ID)

	found, err := repo.FindByNameAndHost("smile", nil)
	require.NoError(t, err)
	assert.Equal(t, "smile", found.Name)
}

func TestEmojiRepository_FindByNameAndHost_Remote(t *testing.T) {
	repo := NewEmojiRepository(testDB)

	host := "remote.example"
	e := &model.Emoji{
		ID:          "e_remote",
		Name:        "smile",
		Host:        &host,
		OriginalURL: "https://remote.example/smile.png",
	}
	require.NoError(t, testDB.Create(e).Error)
	defer cleanupEmoji(t, e.ID)

	found, err := repo.FindByNameAndHost("smile", &host)
	require.NoError(t, err)
	assert.Equal(t, &host, found.Host)
}

func TestEmojiRepository_FindByNameAndHost_NotFound(t *testing.T) {
	repo := NewEmojiRepository(testDB)
	_, err := repo.FindByNameAndHost("ghost", nil)
	assert.Error(t, err)
}

func TestEmojiRepository_FindByNameAndHost_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := NewEmojiRepository(testDB.WithContext(ctx))
	_, err := repo.FindByNameAndHost("x", nil)
	assert.Error(t, err)
}

func TestEmojiRepository_CRUD(t *testing.T) {
	repo := NewEmojiRepository(testDB)

	e := &model.Emoji{ID: "e_crud", Name: "testcrud", OriginalURL: "https://example.com/x.png"}
	require.NoError(t, repo.Create(e))
	defer cleanupEmoji(t, e.ID)

	found, err := repo.FindByID(e.ID)
	require.NoError(t, err)
	assert.Equal(t, "testcrud", found.Name)

	require.NoError(t, repo.UpdateFields(e.ID, map[string]any{"name": "updated"}))
	found, _ = repo.FindByID(e.ID)
	assert.Equal(t, "updated", found.Name)

	require.NoError(t, repo.Delete(e.ID))
	_, err = repo.FindByID(e.ID)
	assert.Error(t, err)
}

func TestEmojiRepository_FindByID_NotFound(t *testing.T) {
	repo := NewEmojiRepository(testDB)
	_, err := repo.FindByID("ghost")
	assert.Error(t, err)
}

func TestEmojiRepository_ListWithFilter(t *testing.T) {
	repo := NewEmojiRepository(testDB)

	e := &model.Emoji{ID: "e_lwf", Name: "filtertest", OriginalURL: "https://example.com/f.png"}
	require.NoError(t, repo.Create(e))
	defer cleanupEmoji(t, e.ID)

	emojis, err := repo.ListWithFilter("filter", "", true, 10, 0)
	require.NoError(t, err)
	assert.NotEmpty(t, emojis)

	// category filter
	emojis, err = repo.ListWithFilter("", "nonexistent", true, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, emojis)

	// pagination
	emojis, err = repo.ListWithFilter("", "", true, 1, 0)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(emojis), 1)
}

func TestEmojiRepository_ListWithFilter_DefaultLimit(t *testing.T) {
	repo := NewEmojiRepository(testDB)
	emojis, err := repo.ListWithFilter("", "", true, 0, 0) // limit=0 → default 50
	require.NoError(t, err)
	_ = emojis
}

func TestEmojiRepository_ListWithFilter_LimitCap(t *testing.T) {
	repo := NewEmojiRepository(testDB)
	emojis, err := repo.ListWithFilter("", "", true, 999, 0)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(emojis), 500)
}

func TestEmojiRepository_ListWithFilter_Offset(t *testing.T) {
	repo := NewEmojiRepository(testDB)
	emojis, err := repo.ListWithFilter("", "", true, 10, 99999)
	require.NoError(t, err)
	assert.Empty(t, emojis)
}

func TestEmojiRepository_ListWithFilter_NonLocal(t *testing.T) {
	repo := NewEmojiRepository(testDB)
	emojis, err := repo.ListWithFilter("", "", false, 10, 0)
	require.NoError(t, err)
	_ = emojis // ローカルフィルタなし
}

func TestEmojiRepository_ListWithFilter_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := NewEmojiRepository(testDB.WithContext(ctx))
	_, err := repo.ListWithFilter("", "", true, 10, 0)
	assert.Error(t, err)
}

func TestEmojiRepository_ListLocal_Empty(t *testing.T) {
	repo := NewEmojiRepository(testDB)
	emojis, err := repo.ListLocal()
	require.NoError(t, err)
	assert.Empty(t, emojis)
}

func TestEmojiRepository_ListLocal_ReturnsLocalOnly(t *testing.T) {
	repo := NewEmojiRepository(testDB)

	local := &model.Emoji{ID: "e_ll", Name: "local_smile", OriginalURL: "https://example.com/s.png"}
	require.NoError(t, testDB.Create(local).Error)
	defer cleanupEmoji(t, local.ID)

	host := "remote.example"
	remote := &model.Emoji{ID: "e_lr", Name: "remote_smile", Host: &host, OriginalURL: "https://remote.example/s.png"}
	require.NoError(t, testDB.Create(remote).Error)
	defer cleanupEmoji(t, remote.ID)

	emojis, err := repo.ListLocal()
	require.NoError(t, err)
	assert.Len(t, emojis, 1)
	assert.Equal(t, "local_smile", emojis[0].Name)
}

func TestEmojiRepository_ListLocal_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := NewEmojiRepository(testDB.WithContext(ctx))
	_, err := repo.ListLocal()
	assert.Error(t, err)
}

func TestEmojiRepository_FindManyByIDs(t *testing.T) {
	repo := NewEmojiRepository(testDB)

	e1 := &model.Emoji{ID: "em_m1", Name: "many1", OriginalURL: "https://x"}
	e2 := &model.Emoji{ID: "em_m2", Name: "many2", OriginalURL: "https://y"}
	require.NoError(t, repo.Create(e1))
	require.NoError(t, repo.Create(e2))
	defer cleanupEmoji(t, e1.ID)
	defer cleanupEmoji(t, e2.ID)

	rows, err := repo.FindManyByIDs([]string{e1.ID, e2.ID, "missing"})
	require.NoError(t, err)
	assert.Len(t, rows, 2)

	empty, err := repo.FindManyByIDs(nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func TestEmojiRepository_UpdateFieldsMany(t *testing.T) {
	repo := NewEmojiRepository(testDB)

	e1 := &model.Emoji{ID: "em_u1", Name: "up1", OriginalURL: "https://x"}
	e2 := &model.Emoji{ID: "em_u2", Name: "up2", OriginalURL: "https://y"}
	require.NoError(t, repo.Create(e1))
	require.NoError(t, repo.Create(e2))
	defer cleanupEmoji(t, e1.ID)
	defer cleanupEmoji(t, e2.ID)

	require.NoError(t, repo.UpdateFieldsMany([]string{e1.ID, e2.ID}, map[string]any{
		"category": "animals",
	}))
	after1, _ := repo.FindByID(e1.ID)
	after2, _ := repo.FindByID(e2.ID)
	require.NotNil(t, after1.Category)
	require.NotNil(t, after2.Category)
	assert.Equal(t, "animals", *after1.Category)
	assert.Equal(t, "animals", *after2.Category)

	// 空 ids / 空 fields は no-op
	require.NoError(t, repo.UpdateFieldsMany(nil, map[string]any{"x": 1}))
	require.NoError(t, repo.UpdateFieldsMany([]string{e1.ID}, map[string]any{}))
}

func TestEmojiRepository_DeleteMany(t *testing.T) {
	repo := NewEmojiRepository(testDB)

	e1 := &model.Emoji{ID: "em_d1", Name: "del1", OriginalURL: "https://x"}
	e2 := &model.Emoji{ID: "em_d2", Name: "del2", OriginalURL: "https://y"}
	require.NoError(t, repo.Create(e1))
	require.NoError(t, repo.Create(e2))
	defer cleanupEmoji(t, e1.ID)
	defer cleanupEmoji(t, e2.ID)

	require.NoError(t, repo.DeleteMany([]string{e1.ID, e2.ID}))
	_, err := repo.FindByID(e1.ID)
	assert.Error(t, err)
	_, err = repo.FindByID(e2.ID)
	assert.Error(t, err)

	// 空 slice は no-op
	require.NoError(t, repo.DeleteMany(nil))
}

func TestEmojiRepository_ListRemoteWithFilter(t *testing.T) {
	repo := NewEmojiRepository(testDB)

	host := "remote.example"
	e1 := &model.Emoji{ID: "em_r1", Name: "rcat", Host: &host, OriginalURL: "https://x"}
	e2 := &model.Emoji{ID: "em_r2", Name: "rdog", Host: &host, OriginalURL: "https://y"}
	require.NoError(t, repo.Create(e1))
	require.NoError(t, repo.Create(e2))
	defer cleanupEmoji(t, e1.ID)
	defer cleanupEmoji(t, e2.ID)

	rows, err := repo.ListRemoteWithFilter("", host, 10, 0)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(rows), 2)

	filtered, err := repo.ListRemoteWithFilter("cat", host, 10, 0)
	require.NoError(t, err)
	assert.Len(t, filtered, 1)
	assert.Equal(t, e1.ID, filtered[0].ID)

	// limit default / cap
	_, err = repo.ListRemoteWithFilter("", host, 0, 0) // default 30
	require.NoError(t, err)
	_, err = repo.ListRemoteWithFilter("", host, 1000, 0) // clamped to 100
	require.NoError(t, err)
}
