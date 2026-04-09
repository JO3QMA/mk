package repository

import (
	"context"
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cleanupAnnouncement(t *testing.T, id string) {
	t.Helper()
	testDB.Exec(`DELETE FROM "announcement_read" WHERE "announcementId" = ?`, id)
	testDB.Exec(`DELETE FROM "announcement" WHERE id = ?`, id)
}

func TestAnnouncementRepository_CRUD(t *testing.T) {
	repo := NewAnnouncementRepository(testDB)

	a := &model.Announcement{ID: "ann_1", Title: "Test", Text: "Hello", Icon: "info", Display: "normal", IsActive: true}
	require.NoError(t, repo.Create(a))
	defer cleanupAnnouncement(t, a.ID)

	found, err := repo.FindByID(a.ID)
	require.NoError(t, err)
	assert.Equal(t, "Test", found.Title)

	// List (active)
	items, err := repo.List(true, 10, 0)
	require.NoError(t, err)
	assert.NotEmpty(t, items)

	// List (all)
	items, err = repo.List(false, 10, 0)
	require.NoError(t, err)
	assert.NotEmpty(t, items)

	// UpdateFields
	require.NoError(t, repo.UpdateFields(a.ID, map[string]any{"title": "Updated"}))
	found, _ = repo.FindByID(a.ID)
	assert.Equal(t, "Updated", found.Title)

	// Delete
	require.NoError(t, repo.Delete(a.ID))
	_, err = repo.FindByID(a.ID)
	assert.Error(t, err)
}

func TestAnnouncementRepository_FindByID_NotFound(t *testing.T) {
	repo := NewAnnouncementRepository(testDB)
	_, err := repo.FindByID("ghost")
	assert.Error(t, err)
}

func TestAnnouncementRepository_List_Pagination(t *testing.T) {
	repo := NewAnnouncementRepository(testDB)
	a1 := &model.Announcement{ID: "ann_p1", Title: "A", Text: "t", Icon: "info", Display: "normal", IsActive: true}
	a2 := &model.Announcement{ID: "ann_p2", Title: "B", Text: "t", Icon: "info", Display: "normal", IsActive: true}
	require.NoError(t, repo.Create(a1))
	require.NoError(t, repo.Create(a2))
	defer cleanupAnnouncement(t, a1.ID)
	defer cleanupAnnouncement(t, a2.ID)

	items, err := repo.List(true, 1, 0)
	require.NoError(t, err)
	assert.Len(t, items, 1)

	items, err = repo.List(true, 10, 1)
	require.NoError(t, err)
	assert.NotEmpty(t, items)

	// default limit
	items, err = repo.List(true, 0, 0)
	require.NoError(t, err)
	assert.NotEmpty(t, items)

	// limit cap
	items, err = repo.List(true, 999, 0)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(items), 100)
}

func TestAnnouncementRepository_List_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := NewAnnouncementRepository(testDB.WithContext(ctx))
	_, err := repo.List(true, 10, 0)
	assert.Error(t, err)
}

func TestAnnouncementRepository_ReadManagement(t *testing.T) {
	repo := NewAnnouncementRepository(testDB)
	createTestUser(t, "ann_reader")

	a := &model.Announcement{ID: "ann_r1", Title: "R", Text: "t", Icon: "info", Display: "normal", IsActive: true}
	require.NoError(t, repo.Create(a))
	defer cleanupAnnouncement(t, a.ID)

	// Not read yet
	read, err := repo.IsRead("ann_reader", a.ID)
	require.NoError(t, err)
	assert.False(t, read)

	// Unread list
	unread, err := repo.UnreadForUser("ann_reader")
	require.NoError(t, err)
	assert.NotEmpty(t, unread)

	// Mark read
	require.NoError(t, repo.MarkRead(&model.AnnouncementRead{ID: "ar_1", UserID: "ann_reader", AnnouncementID: a.ID}))

	read, err = repo.IsRead("ann_reader", a.ID)
	require.NoError(t, err)
	assert.True(t, read)

	unread, err = repo.UnreadForUser("ann_reader")
	require.NoError(t, err)
	assert.Empty(t, unread)
}

func TestAnnouncementRepository_UnreadForUser_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := NewAnnouncementRepository(testDB.WithContext(ctx))
	_, err := repo.UnreadForUser("x")
	assert.Error(t, err)
}

func TestAnnouncementRepository_IsRead_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := NewAnnouncementRepository(testDB.WithContext(ctx))
	_, err := repo.IsRead("x", "y")
	assert.Error(t, err)
}
