package repository

import (
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cleanupAd(t *testing.T, id string) {
	t.Helper()
	testDB.Exec(`DELETE FROM "ad" WHERE id = ?`, id)
}

func TestAdRepository_ListActive(t *testing.T) {
	repo := NewAdRepository(testDB)
	now := time.Now()

	active := &model.Ad{
		ID:        "ad_active",
		URL:       "https://example.com/a",
		ImageURL:  "https://example.com/a.png",
		Place:     "square",
		Priority:  "middle",
		Ratio:     1,
		StartsAt:  now.Add(-1 * time.Hour),
		ExpiresAt: now.Add(1 * time.Hour),
	}
	expired := &model.Ad{
		ID:        "ad_expired",
		URL:       "https://example.com/b",
		ImageURL:  "https://example.com/b.png",
		Place:     "square",
		Priority:  "middle",
		Ratio:     1,
		StartsAt:  now.Add(-2 * time.Hour),
		ExpiresAt: now.Add(-1 * time.Hour),
	}
	future := &model.Ad{
		ID:        "ad_future",
		URL:       "https://example.com/c",
		ImageURL:  "https://example.com/c.png",
		Place:     "square",
		Priority:  "middle",
		Ratio:     1,
		StartsAt:  now.Add(1 * time.Hour),
		ExpiresAt: now.Add(2 * time.Hour),
	}

	require.NoError(t, testDB.Create(active).Error)
	defer cleanupAd(t, active.ID)
	require.NoError(t, testDB.Create(expired).Error)
	defer cleanupAd(t, expired.ID)
	require.NoError(t, testDB.Create(future).Error)
	defer cleanupAd(t, future.ID)

	got, err := repo.ListActive(now)
	require.NoError(t, err)

	ids := make(map[string]bool, len(got))
	for _, a := range got {
		ids[a.ID] = true
	}
	assert.True(t, ids["ad_active"], "active ad should be returned")
	assert.False(t, ids["ad_expired"], "expired ad should not be returned")
	assert.False(t, ids["ad_future"], "future ad should not be returned")
}

func TestAdRepository_CRUD(t *testing.T) {
	repo := NewAdRepository(testDB)
	now := time.Now()

	a := &model.Ad{
		ID:        "ad_crud",
		URL:       "https://example.com/x",
		ImageURL:  "https://example.com/x.png",
		Place:     "square",
		Priority:  "middle",
		Ratio:     1,
		StartsAt:  now,
		ExpiresAt: now.Add(time.Hour),
	}
	require.NoError(t, repo.Create(a))
	defer cleanupAd(t, a.ID)

	got, err := repo.FindByID(a.ID)
	require.NoError(t, err)
	assert.Equal(t, "square", got.Place)

	_, err = repo.FindByID("ghost_ad")
	assert.Error(t, err)

	rows, err := repo.List(10, 0)
	require.NoError(t, err)
	assert.NotEmpty(t, rows)

	require.NoError(t, repo.UpdateFields(a.ID, map[string]any{
		"url":      "https://example.com/renamed",
		"priority": "high",
	}))
	got, err = repo.FindByID(a.ID)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/renamed", got.URL)
	assert.Equal(t, "high", got.Priority)

	require.NoError(t, repo.UpdateFields(a.ID, map[string]any{}))

	require.NoError(t, repo.Delete(a.ID))
	_, err = repo.FindByID(a.ID)
	assert.Error(t, err)
}

func TestAdRepository_ListDefaults(t *testing.T) {
	repo := NewAdRepository(testDB)
	// 負数 / 0 は default に丸められて成功するだけを確認する。
	_, err := repo.List(0, -1)
	require.NoError(t, err)
}
