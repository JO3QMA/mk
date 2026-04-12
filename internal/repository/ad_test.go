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
