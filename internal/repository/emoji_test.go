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
