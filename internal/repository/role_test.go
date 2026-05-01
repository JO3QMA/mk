package repository

import (
	"context"
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func cancelledDB(t *testing.T) *gorm.DB {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return testDB.WithContext(ctx)
}

func cleanupRole(t *testing.T, id string) {
	t.Helper()
	testDB.Exec(`DELETE FROM "role_assignment" WHERE "roleId" = ?`, id)
	testDB.Exec(`DELETE FROM "role" WHERE id = ?`, id)
}

func createTestUser(t *testing.T, id string) {
	t.Helper()
	testDB.Exec(`INSERT INTO "user" (id, username, "usernameLower", "avatarDecorations") VALUES (?, ?, ?, '[]') ON CONFLICT DO NOTHING`, id, id, id)
	t.Cleanup(func() { testDB.Exec(`DELETE FROM "user" WHERE id = ?`, id) })
}

// --- RoleRepository ---

func TestRoleRepository_CRUD(t *testing.T) {
	repo := NewRoleRepository(testDB)

	now := time.Now()
	role := &model.Role{
		ID:              "role_test_1",
		UpdatedAt:       now,
		LastUsedAt:      now,
		Name:            "Test Admin",
		Description:     "Test admin role",
		IsAdministrator: true,
		IsPublic:        true,
		Target:          model.RoleTargetManual,
		Policies:        datatypes.JSON([]byte("{}")),
		CondFormula:     datatypes.JSON([]byte("{}")),
	}
	require.NoError(t, repo.Create(role))
	defer cleanupRole(t, role.ID)

	// FindByID
	found, err := repo.FindByID(role.ID)
	require.NoError(t, err)
	assert.Equal(t, "Test Admin", found.Name)
	assert.True(t, found.IsAdministrator)

	// List
	roles, err := repo.List()
	require.NoError(t, err)
	assert.NotEmpty(t, roles)

	// UpdateFields
	require.NoError(t, repo.UpdateFields(role.ID, map[string]any{"name": "Updated"}))
	updated, err := repo.FindByID(role.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated", updated.Name)

	// Delete
	require.NoError(t, repo.Delete(role.ID))
	_, err = repo.FindByID(role.ID)
	assert.Error(t, err)
}

func TestRoleRepository_FindByID_NotFound(t *testing.T) {
	repo := NewRoleRepository(testDB)
	_, err := repo.FindByID("ghost")
	assert.Error(t, err)
}

// --- RoleAssignmentRepository ---

func TestRoleAssignmentRepository_CRUD(t *testing.T) {
	roleRepo := NewRoleRepository(testDB)
	assignRepo := NewRoleAssignmentRepository(testDB)

	now := time.Now()
	role := &model.Role{
		ID: "role_a_test", UpdatedAt: now, LastUsedAt: now, Name: "Mod",
		Target: model.RoleTargetManual, Policies: datatypes.JSON([]byte("{}")),
		CondFormula: datatypes.JSON([]byte("{}")), IsModerator: true,
	}
	require.NoError(t, roleRepo.Create(role))
	defer cleanupRole(t, role.ID)

	createTestUser(t, "ra_user1")

	// Create
	a := &model.RoleAssignment{ID: "ra_1", UserID: "ra_user1", RoleID: role.ID}
	require.NoError(t, assignRepo.Create(a))

	// Exists
	exists, err := assignRepo.Exists("ra_user1", role.ID)
	require.NoError(t, err)
	assert.True(t, exists)

	// ListByUser (with role preload)
	assignments, err := assignRepo.ListByUser("ra_user1")
	require.NoError(t, err)
	assert.Len(t, assignments, 1)
	assert.NotNil(t, assignments[0].Role)
	assert.Equal(t, "Mod", assignments[0].Role.Name)

	// ListByRole (with user preload)
	byRole, err := assignRepo.ListByRole(role.ID, "", "", 10)
	require.NoError(t, err)
	assert.Len(t, byRole, 1)

	// Delete
	require.NoError(t, assignRepo.Delete("ra_user1", role.ID))
	exists, err = assignRepo.Exists("ra_user1", role.ID)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestRoleRepository_List_Error(t *testing.T) {
	db := cancelledDB(t)
	repo := NewRoleRepository(db)
	_, err := repo.List()
	assert.Error(t, err)
}

func TestRoleAssignmentRepository_ListByUser_Error(t *testing.T) {
	db := cancelledDB(t)
	repo := NewRoleAssignmentRepository(db)
	_, err := repo.ListByUser("x")
	assert.Error(t, err)
}

func TestRoleAssignmentRepository_ListByRole_WithPagination(t *testing.T) {
	roleRepo := NewRoleRepository(testDB)
	assignRepo := NewRoleAssignmentRepository(testDB)

	now := time.Now()
	role := &model.Role{
		ID: "role_pag_test", UpdatedAt: now, LastUsedAt: now, Name: "Pag",
		Target: model.RoleTargetManual, Policies: datatypes.JSON([]byte("{}")),
		CondFormula: datatypes.JSON([]byte("{}")),
	}
	require.NoError(t, roleRepo.Create(role))
	defer cleanupRole(t, role.ID)

	createTestUser(t, "pag_u1")
	createTestUser(t, "pag_u2")
	require.NoError(t, assignRepo.Create(&model.RoleAssignment{ID: "pag_a1", UserID: "pag_u1", RoleID: role.ID}))
	require.NoError(t, assignRepo.Create(&model.RoleAssignment{ID: "pag_a2", UserID: "pag_u2", RoleID: role.ID}))

	// limit=1
	result, err := assignRepo.ListByRole(role.ID, "", "", 1)
	require.NoError(t, err)
	assert.Len(t, result, 1)

	// untilID で keyset cursor (id DESC で pag_a2 → pag_a1 と進む途中)
	result, err = assignRepo.ListByRole(role.ID, "pag_a2", "", 10)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "pag_a1", result[0].ID)

	// sinceID で ASC cursor (pag_a1 の次は pag_a2)
	result, err = assignRepo.ListByRole(role.ID, "", "pag_a1", 10)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "pag_a2", result[0].ID)
}

func TestRoleAssignmentRepository_ListByRole_Error(t *testing.T) {
	db := cancelledDB(t)
	repo := NewRoleAssignmentRepository(db)
	_, err := repo.ListByRole("x", "", "", 10)
	assert.Error(t, err)
}

func TestRoleAssignmentRepository_Exists_Error(t *testing.T) {
	db := cancelledDB(t)
	repo := NewRoleAssignmentRepository(db)
	_, err := repo.Exists("x", "y")
	assert.Error(t, err)
}

func TestRoleAssignmentRepository_ExpiredExcluded(t *testing.T) {
	roleRepo := NewRoleRepository(testDB)
	assignRepo := NewRoleAssignmentRepository(testDB)

	now := time.Now()
	role := &model.Role{
		ID: "role_exp_test", UpdatedAt: now, LastUsedAt: now, Name: "Temp",
		Target: model.RoleTargetManual, Policies: datatypes.JSON([]byte("{}")),
		CondFormula: datatypes.JSON([]byte("{}")),
	}
	require.NoError(t, roleRepo.Create(role))
	defer cleanupRole(t, role.ID)

	createTestUser(t, "ra_user2")

	expired := now.Add(-1 * time.Hour)
	a := &model.RoleAssignment{ID: "ra_exp", UserID: "ra_user2", RoleID: role.ID, ExpiresAt: &expired}
	require.NoError(t, assignRepo.Create(a))

	// ListByUser — expired assignment excluded
	assignments, err := assignRepo.ListByUser("ra_user2")
	require.NoError(t, err)
	assert.Empty(t, assignments)

	// Exists — expired → false
	exists, err := assignRepo.Exists("ra_user2", role.ID)
	require.NoError(t, err)
	assert.False(t, exists)

	// ListByRole — expired excluded
	byRole, err := assignRepo.ListByRole(role.ID, "", "", 10)
	require.NoError(t, err)
	assert.Empty(t, byRole)

	// cleanup
	testDB.Exec(`DELETE FROM "role_assignment" WHERE id = ?`, a.ID)
}
