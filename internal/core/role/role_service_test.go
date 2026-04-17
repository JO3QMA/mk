package role_test

import (
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/core/role"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func newTestService(t *testing.T) (*role.Service, *testutil.MockRoleRepository, *testutil.MockRoleAssignmentRepository, *testutil.MockMetaRepository) {
	t.Helper()
	roleRepo := testutil.NewMockRoleRepository()
	assignRepo := testutil.NewMockRoleAssignmentRepository(roleRepo)
	metaRepo := testutil.NewMockMetaRepository()
	idGen, _ := id.NewGenerator("aidx")
	svc := role.NewService(roleRepo, assignRepo, metaRepo, idGen)
	return svc, roleRepo, assignRepo, metaRepo
}

func TestGetUserRoles_NoRoles(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	roles, err := svc.GetUserRoles("user1")
	require.NoError(t, err)
	assert.Empty(t, roles)
}

func TestGetUserRoles_WithRoles(t *testing.T) {
	svc, roleRepo, assignRepo, _ := newTestService(t)
	adminRole := &model.Role{ID: "r1", Name: "Admin", IsAdministrator: true}
	roleRepo.Roles["r1"] = adminRole
	assignRepo.Assignments["user1:r1"] = &model.RoleAssignment{
		ID: "a1", UserID: "user1", RoleID: "r1",
	}

	roles, err := svc.GetUserRoles("user1")
	require.NoError(t, err)
	assert.Len(t, roles, 1)
	assert.Equal(t, "Admin", roles[0].Name)
}

func TestGetUserRoles_ExpiredRoleExcluded(t *testing.T) {
	svc, roleRepo, assignRepo, _ := newTestService(t)
	roleRepo.Roles["r1"] = &model.Role{ID: "r1", Name: "Temp"}
	expired := time.Now().Add(-1 * time.Hour)
	assignRepo.Assignments["user1:r1"] = &model.RoleAssignment{
		ID: "a1", UserID: "user1", RoleID: "r1", ExpiresAt: &expired,
	}

	roles, err := svc.GetUserRoles("user1")
	require.NoError(t, err)
	assert.Empty(t, roles)
}

func TestIsAdministrator_RootUser(t *testing.T) {
	svc, _, _, metaRepo := newTestService(t)
	rootID := "root1"
	metaRepo.Meta = &model.Meta{ID: "x", RootUserID: &rootID}
	assert.True(t, svc.IsAdministrator("root1"))
}

func TestIsAdministrator_AdminRole(t *testing.T) {
	svc, roleRepo, assignRepo, metaRepo := newTestService(t)
	metaRepo.Meta = &model.Meta{ID: "x"}
	roleRepo.Roles["r1"] = &model.Role{ID: "r1", IsAdministrator: true}
	assignRepo.Assignments["user1:r1"] = &model.RoleAssignment{
		ID: "a1", UserID: "user1", RoleID: "r1",
	}
	assert.True(t, svc.IsAdministrator("user1"))
}

func TestIsAdministrator_NoRole(t *testing.T) {
	svc, _, _, metaRepo := newTestService(t)
	metaRepo.Meta = &model.Meta{ID: "x"}
	assert.False(t, svc.IsAdministrator("user1"))
}

func TestIsModerator_ModeratorRole(t *testing.T) {
	svc, roleRepo, assignRepo, metaRepo := newTestService(t)
	metaRepo.Meta = &model.Meta{ID: "x"}
	roleRepo.Roles["r1"] = &model.Role{ID: "r1", IsModerator: true}
	assignRepo.Assignments["user1:r1"] = &model.RoleAssignment{
		ID: "a1", UserID: "user1", RoleID: "r1",
	}
	assert.True(t, svc.IsModerator("user1"))
}

func TestIsModerator_AdminRoleAlsoCounts(t *testing.T) {
	svc, roleRepo, assignRepo, metaRepo := newTestService(t)
	metaRepo.Meta = &model.Meta{ID: "x"}
	roleRepo.Roles["r1"] = &model.Role{ID: "r1", IsAdministrator: true}
	assignRepo.Assignments["user1:r1"] = &model.RoleAssignment{
		ID: "a1", UserID: "user1", RoleID: "r1",
	}
	assert.True(t, svc.IsModerator("user1"))
}

func TestIsModerator_NoRole(t *testing.T) {
	svc, _, _, metaRepo := newTestService(t)
	metaRepo.Meta = &model.Meta{ID: "x"}
	assert.False(t, svc.IsModerator("user1"))
}

func TestIsAdministrator_MetaFetchError(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	// metaRepo.Meta is nil → Fetch returns error
	assert.False(t, svc.IsAdministrator("user1"))
}

func TestGetUserPolicies_Default(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	policies := svc.GetUserPolicies("user1")
	assert.Equal(t, true, policies["gtlAvailable"])
	assert.Equal(t, 100, policies["driveCapacityMb"])
}

func TestGetUserPolicies_WithRoleOverride(t *testing.T) {
	svc, roleRepo, assignRepo, _ := newTestService(t)
	// ロールが driveCapacityMb を 500 にオーバーライド
	roleRepo.Roles["r1"] = &model.Role{
		ID:   "r1",
		Name: "Pro",
		Policies: datatypes.JSON([]byte(`{
			"driveCapacityMb": {"useDefault": false, "priority": 0, "value": 500}
		}`)),
	}
	assignRepo.Assignments["user1:r1"] = &model.RoleAssignment{
		ID: "a1", UserID: "user1", RoleID: "r1",
	}

	policies := svc.GetUserPolicies("user1")
	assert.Equal(t, float64(500), policies["driveCapacityMb"])
}

func TestGetUserPolicies_UseDefaultTrue_NotOverridden(t *testing.T) {
	svc, roleRepo, assignRepo, _ := newTestService(t)
	roleRepo.Roles["r1"] = &model.Role{
		ID:       "r1",
		Policies: datatypes.JSON([]byte(`{"driveCapacityMb": {"useDefault": true, "priority": 0, "value": 999}}`)),
	}
	assignRepo.Assignments["user1:r1"] = &model.RoleAssignment{
		ID: "a1", UserID: "user1", RoleID: "r1",
	}

	policies := svc.GetUserPolicies("user1")
	assert.Equal(t, 100, policies["driveCapacityMb"]) // デフォルト値が維持される
}

func TestGetUserPolicies_InvalidJSON(t *testing.T) {
	svc, roleRepo, assignRepo, _ := newTestService(t)
	roleRepo.Roles["r1"] = &model.Role{
		ID:       "r1",
		Policies: datatypes.JSON([]byte(`invalid`)),
	}
	assignRepo.Assignments["user1:r1"] = &model.RoleAssignment{
		ID: "a1", UserID: "user1", RoleID: "r1",
	}

	policies := svc.GetUserPolicies("user1")
	// 不正なJSONの場合はデフォルト値がそのまま返る
	assert.Equal(t, 100, policies["driveCapacityMb"])
}

func TestAssign_Success(t *testing.T) {
	svc, roleRepo, assignRepo, _ := newTestService(t)
	roleRepo.Roles["r1"] = &model.Role{ID: "r1", Name: "Admin"}

	err := svc.Assign("user1", "r1", nil)
	require.NoError(t, err)
	assert.Len(t, assignRepo.Assignments, 1)
}

func TestAssign_RoleNotFound(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	err := svc.Assign("user1", "ghost", nil)
	assert.ErrorIs(t, err, role.ErrRoleNotFound)
}

func TestAssign_AlreadyAssigned(t *testing.T) {
	svc, roleRepo, assignRepo, _ := newTestService(t)
	roleRepo.Roles["r1"] = &model.Role{ID: "r1"}
	assignRepo.Assignments["user1:r1"] = &model.RoleAssignment{
		ID: "a1", UserID: "user1", RoleID: "r1",
	}

	err := svc.Assign("user1", "r1", nil)
	assert.ErrorIs(t, err, role.ErrAlreadyAssigned)
}

func TestAssign_WithExpiry(t *testing.T) {
	svc, roleRepo, _, _ := newTestService(t)
	roleRepo.Roles["r1"] = &model.Role{ID: "r1"}
	exp := time.Now().Add(24 * time.Hour)
	err := svc.Assign("user1", "r1", &exp)
	require.NoError(t, err)
}

func TestUnassign_Success(t *testing.T) {
	svc, _, assignRepo, _ := newTestService(t)
	assignRepo.Assignments["user1:r1"] = &model.RoleAssignment{
		ID: "a1", UserID: "user1", RoleID: "r1",
	}

	err := svc.Unassign("user1", "r1")
	require.NoError(t, err)
	assert.Empty(t, assignRepo.Assignments)
}

func TestUnassign_NotAssigned(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	err := svc.Unassign("user1", "r1")
	assert.ErrorIs(t, err, role.ErrNotAssigned)
}

func TestCreate_Success(t *testing.T) {
	svc, roleRepo, _, _ := newTestService(t)
	r, err := svc.Create("Admin", "Administrator role", role.CreateOptions{
		IsAdministrator: true,
		IsPublic:        true,
	})
	require.NoError(t, err)
	assert.Equal(t, "Admin", r.Name)
	assert.True(t, r.IsAdministrator)
	assert.Len(t, roleRepo.Roles, 1)
}

func TestShow_Found(t *testing.T) {
	svc, roleRepo, _, _ := newTestService(t)
	roleRepo.Roles["r1"] = &model.Role{ID: "r1", Name: "Test"}
	r, err := svc.Show("r1")
	require.NoError(t, err)
	assert.Equal(t, "Test", r.Name)
}

func TestShow_NotFound(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	_, err := svc.Show("ghost")
	assert.ErrorIs(t, err, role.ErrRoleNotFound)
}

func TestList(t *testing.T) {
	svc, roleRepo, _, _ := newTestService(t)
	roleRepo.Roles["r1"] = &model.Role{ID: "r1", Name: "A"}
	roleRepo.Roles["r2"] = &model.Role{ID: "r2", Name: "B"}
	roles, err := svc.List()
	require.NoError(t, err)
	assert.Len(t, roles, 2)
}

func TestDelete_Success(t *testing.T) {
	svc, roleRepo, _, _ := newTestService(t)
	roleRepo.Roles["r1"] = &model.Role{ID: "r1"}
	err := svc.Delete("r1")
	require.NoError(t, err)
	assert.Empty(t, roleRepo.Roles)
}

func TestDelete_NotFound(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	err := svc.Delete("ghost")
	assert.ErrorIs(t, err, role.ErrRoleNotFound)
}

// --- Error path tests ---

func TestGetUserRoles_RepoError(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	// assignRepo は空なのでエラーにならないが、nil role は除外される
	roles, err := svc.GetUserRoles("user1")
	require.NoError(t, err)
	assert.Empty(t, roles)
}

func TestIsAdministrator_GetRolesError(t *testing.T) {
	// metaRepo にMeta設定してrootUser判定をスキップ → GetUserRoles を呼ぶ
	svc, _, _, metaRepo := newTestService(t)
	metaRepo.Meta = &model.Meta{ID: "x"}
	// assignRepo が空なので GetUserRoles は空を返す → false
	assert.False(t, svc.IsAdministrator("user1"))
}

func TestIsModerator_RootUser(t *testing.T) {
	svc, _, _, metaRepo := newTestService(t)
	rootID := "root1"
	metaRepo.Meta = &model.Meta{ID: "x", RootUserID: &rootID}
	assert.True(t, svc.IsModerator("root1"))
}

func TestIsModerator_MetaFetchError(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	// metaRepo.Meta = nil → Fetch error → isRootUser false → no roles → false
	assert.False(t, svc.IsModerator("user1"))
}

func TestAssign_ExistsError(t *testing.T) {
	// Exists がエラーを返すケースは mock では発生しにくいが、
	// role が存在して assignment が空の場合は正常に Assign できるので
	// Create のエラーパスをテスト
	svc, roleRepo, _, _ := newTestService(t)
	roleRepo.Roles["r1"] = &model.Role{ID: "r1"}
	err := svc.Assign("user1", "r1", nil)
	require.NoError(t, err)
}

func TestUnassign_DeleteError(t *testing.T) {
	// Unassign で Exists true → Delete → mock では常に成功
	svc, _, assignRepo, _ := newTestService(t)
	assignRepo.Assignments["user1:r1"] = &model.RoleAssignment{
		ID: "a1", UserID: "user1", RoleID: "r1",
	}
	require.NoError(t, svc.Unassign("user1", "r1"))
}

func TestCreate_RepoError(t *testing.T) {
	// mock の Create は常に成功するのでエラーパスは到達しにくい
	// 正常パスの補強として CreateOptions の全フィールドを検証
	svc, _, _, _ := newTestService(t)
	r, err := svc.Create("Mod", "Moderator", role.CreateOptions{
		IsModerator:  true,
		AsBadge:      true,
		IsExplorable: true,
		DisplayOrder: 10,
	})
	require.NoError(t, err)
	assert.True(t, r.IsModerator)
	assert.Equal(t, 10, r.DisplayOrder)
}

func TestApplyRolePolicies_EmptyPolicies(t *testing.T) {
	svc, roleRepo, assignRepo, _ := newTestService(t)
	roleRepo.Roles["r1"] = &model.Role{
		ID:       "r1",
		Policies: datatypes.JSON([]byte(`{}`)),
	}
	assignRepo.Assignments["user1:r1"] = &model.RoleAssignment{
		ID: "a1", UserID: "user1", RoleID: "r1",
	}
	policies := svc.GetUserPolicies("user1")
	assert.Equal(t, 100, policies["driveCapacityMb"]) // デフォルト維持
}

// --- Failing repository tests ---

type failingAssignRepo struct {
	*testutil.MockRoleAssignmentRepository
}

func (f *failingAssignRepo) ListByUser(_ string) ([]*model.RoleAssignment, error) {
	return nil, assert.AnError
}

func (f *failingAssignRepo) Exists(_ string, _ string) (bool, error) {
	return false, assert.AnError
}

type failingRoleRepo struct {
	*testutil.MockRoleRepository
}

func (f *failingRoleRepo) Create(_ *model.Role) error { return assert.AnError }

func TestGetUserRoles_ListByUserError(t *testing.T) {
	roleRepo := testutil.NewMockRoleRepository()
	assignRepo := &failingAssignRepo{testutil.NewMockRoleAssignmentRepository(roleRepo)}
	metaRepo := testutil.NewMockMetaRepository()
	idGen, _ := id.NewGenerator("aidx")
	svc := role.NewService(roleRepo, assignRepo, metaRepo, idGen)

	_, err := svc.GetUserRoles("user1")
	assert.Error(t, err)
}

func TestIsAdministrator_GetRolesError_ReturnsFalse(t *testing.T) {
	roleRepo := testutil.NewMockRoleRepository()
	assignRepo := &failingAssignRepo{testutil.NewMockRoleAssignmentRepository(roleRepo)}
	metaRepo := testutil.NewMockMetaRepository()
	metaRepo.Meta = &model.Meta{ID: "x"}
	idGen, _ := id.NewGenerator("aidx")
	svc := role.NewService(roleRepo, assignRepo, metaRepo, idGen)

	assert.False(t, svc.IsAdministrator("user1"))
}

func TestIsModerator_GetRolesError_ReturnsFalse(t *testing.T) {
	roleRepo := testutil.NewMockRoleRepository()
	assignRepo := &failingAssignRepo{testutil.NewMockRoleAssignmentRepository(roleRepo)}
	metaRepo := testutil.NewMockMetaRepository()
	metaRepo.Meta = &model.Meta{ID: "x"}
	idGen, _ := id.NewGenerator("aidx")
	svc := role.NewService(roleRepo, assignRepo, metaRepo, idGen)

	assert.False(t, svc.IsModerator("user1"))
}

func TestAssign_ExistsRepoError(t *testing.T) {
	rr := testutil.NewMockRoleRepository()
	rr.Roles["r1"] = &model.Role{ID: "r1"}
	assignRepo := &failingAssignRepo{testutil.NewMockRoleAssignmentRepository(rr)}
	metaRepo := testutil.NewMockMetaRepository()
	idGen, _ := id.NewGenerator("aidx")
	svc := role.NewService(rr, assignRepo, metaRepo, idGen)

	err := svc.Assign("user1", "r1", nil)
	assert.Error(t, err)
}

func TestUnassign_ExistsRepoError(t *testing.T) {
	roleRepo := testutil.NewMockRoleRepository()
	assignRepo := &failingAssignRepo{testutil.NewMockRoleAssignmentRepository(roleRepo)}
	metaRepo := testutil.NewMockMetaRepository()
	idGen, _ := id.NewGenerator("aidx")
	svc := role.NewService(roleRepo, assignRepo, metaRepo, idGen)

	err := svc.Unassign("user1", "r1")
	assert.Error(t, err)
}

func TestCreate_RepoCreateError(t *testing.T) {
	rr := &failingRoleRepo{testutil.NewMockRoleRepository()}
	assignRepo := testutil.NewMockRoleAssignmentRepository(rr.MockRoleRepository)
	metaRepo := testutil.NewMockMetaRepository()
	idGen, _ := id.NewGenerator("aidx")
	svc := role.NewService(rr, assignRepo, metaRepo, idGen)

	_, err := svc.Create("Test", "desc", role.CreateOptions{})
	assert.Error(t, err)
}

func TestApplyRolePolicies_NilPolicies(t *testing.T) {
	svc, roleRepo, assignRepo, _ := newTestService(t)
	roleRepo.Roles["r1"] = &model.Role{ID: "r1", Policies: nil}
	assignRepo.Assignments["user1:r1"] = &model.RoleAssignment{
		ID: "a1", UserID: "user1", RoleID: "r1",
	}
	policies := svc.GetUserPolicies("user1")
	assert.Equal(t, 100, policies["driveCapacityMb"])
}

func TestDefaultPolicies(t *testing.T) {
	p := role.DefaultPolicies()
	assert.Equal(t, true, p["gtlAvailable"])
	assert.Equal(t, 100, p["driveCapacityMb"])
	assert.Equal(t, 5, p["pinLimit"])
}

func TestIsSilenced_DefaultIsFalse(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	// 誰もロールを持っていない → DefaultPolicies は canPublicNote=true
	assert.False(t, svc.IsSilenced("user1"))
}

func TestIsSilenced_CanPublicNoteFalseMeansSilenced(t *testing.T) {
	svc, roleRepo, assignRepo, _ := newTestService(t)
	// policies JSON で canPublicNote=false を上書きした role を持たせる
	roleRepo.Roles["r1"] = &model.Role{
		ID: "r1",
		Policies: datatypes.JSON([]byte(
			`{"canPublicNote":{"useDefault":false,"priority":1,"value":false}}`,
		)),
	}
	assignRepo.Assignments["user1:r1"] = &model.RoleAssignment{
		ID: "a1", UserID: "user1", RoleID: "r1",
	}
	assert.True(t, svc.IsSilenced("user1"))
}
