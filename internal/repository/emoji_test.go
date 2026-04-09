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
