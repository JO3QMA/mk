package repository

import (
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGalleryRepository_ListByUser(t *testing.T) {
	repo := NewGalleryRepository(testDB)
	seedUser(t, "gp_u1")
	t.Cleanup(func() { testDB.Exec(`DELETE FROM "gallery_post" WHERE "userId" = ?`, "gp_u1") })

	for _, id := range []string{"gp_1", "gp_2", "gp_3"} {
		require.NoError(t, testDB.Create(&model.GalleryPost{
			ID: id, UserID: "gp_u1", Title: "t_" + id, UpdatedAt: time.Now(),
		}).Error)
	}

	posts, err := repo.ListByUser("gp_u1", 10, 0)
	require.NoError(t, err)
	assert.Len(t, posts, 3)

	// pagination: offset > 0
	posts, err = repo.ListByUser("gp_u1", 10, 1)
	require.NoError(t, err)
	assert.Len(t, posts, 2)

	// clampLimit: 0 -> 10 (but only 3 rows so limit is not the limiting factor)
	posts, err = repo.ListByUser("gp_u1", 0, 0)
	require.NoError(t, err)
	assert.Len(t, posts, 3)

	// clampLimit: >100 -> 100
	posts, err = repo.ListByUser("gp_u1", 9999, 0)
	require.NoError(t, err)
	assert.Len(t, posts, 3)
}

func TestGalleryRepository_ListLikesByUser(t *testing.T) {
	repo := NewGalleryRepository(testDB)
	seedUser(t, "gl_u1")
	// postはlikeのFKなので先に作る
	require.NoError(t, testDB.Create(&model.GalleryPost{ID: "gl_p1", UserID: "gl_u1", Title: "t", UpdatedAt: time.Now()}).Error)
	t.Cleanup(func() {
		testDB.Exec(`DELETE FROM "gallery_like" WHERE "userId" = ?`, "gl_u1")
		testDB.Exec(`DELETE FROM "gallery_post" WHERE id = ?`, "gl_p1")
	})

	require.NoError(t, testDB.Create(&model.GalleryLike{ID: "gl_l1", UserID: "gl_u1", PostID: "gl_p1"}).Error)

	likes, err := repo.ListLikesByUser("gl_u1", 10, 0)
	require.NoError(t, err)
	assert.Len(t, likes, 1)

	// offset > 0
	likes, err = repo.ListLikesByUser("gl_u1", 10, 1)
	require.NoError(t, err)
	assert.Empty(t, likes)
}

func TestGalleryRepository_EmptyList(t *testing.T) {
	repo := NewGalleryRepository(testDB)
	posts, err := repo.ListByUser("gp_nonexistent", 10, 0)
	require.NoError(t, err)
	assert.Empty(t, posts)

	likes, err := repo.ListLikesByUser("gp_nonexistent", 10, 0)
	require.NoError(t, err)
	assert.Empty(t, likes)
}
