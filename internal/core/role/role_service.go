package role

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

var (
	// ErrRoleNotFound is returned when the target role does not exist.
	ErrRoleNotFound = errors.New("role not found")
	// ErrAlreadyAssigned is returned when the user already has the role.
	ErrAlreadyAssigned = errors.New("role already assigned")
	// ErrNotAssigned is returned when the user does not have the role.
	ErrNotAssigned = errors.New("role not assigned")
)

// Service manages roles and role assignments.
type Service struct {
	roleRepo       repository.RoleRepository
	assignmentRepo repository.RoleAssignmentRepository
	metaRepo       repository.MetaRepository
	idGen          id.Generator
}

// NewService constructs a RoleService.
func NewService(
	roleRepo repository.RoleRepository,
	assignmentRepo repository.RoleAssignmentRepository,
	metaRepo repository.MetaRepository,
	idGen id.Generator,
) *Service {
	return &Service{
		roleRepo:       roleRepo,
		assignmentRepo: assignmentRepo,
		metaRepo:       metaRepo,
		idGen:          idGen,
	}
}

// GetUserRoles returns all active (non-expired) roles assigned to the user.
func (s *Service) GetUserRoles(userID string) ([]*model.Role, error) {
	assignments, err := s.assignmentRepo.ListByUser(userID)
	if err != nil {
		return nil, err
	}
	roles := make([]*model.Role, 0, len(assignments))
	for _, a := range assignments {
		if a.Role != nil {
			roles = append(roles, a.Role)
		}
	}
	return roles, nil
}

// isRootUser checks if the user is the root user (meta.rootUserId).
func (s *Service) isRootUser(userID string) bool {
	meta, err := s.metaRepo.Fetch()
	if err != nil {
		return false
	}
	return meta.RootUserID != nil && *meta.RootUserID == userID
}

// IsAdministrator checks if the user has any administrator role or is root.
func (s *Service) IsAdministrator(userID string) bool {
	if s.isRootUser(userID) {
		return true
	}
	roles, err := s.GetUserRoles(userID)
	if err != nil {
		return false
	}
	for _, r := range roles {
		if r.IsAdministrator {
			return true
		}
	}
	return false
}

// IsModerator checks if the user has any moderator/admin role or is root.
func (s *Service) IsModerator(userID string) bool {
	if s.isRootUser(userID) {
		return true
	}
	roles, err := s.GetUserRoles(userID)
	if err != nil {
		return false
	}
	for _, r := range roles {
		if r.IsModerator || r.IsAdministrator {
			return true
		}
	}
	return false
}

// GetUserPolicies computes merged policies for a user based on assigned roles.
// デフォルトポリシーにロール固有のポリシーをマージする。
func (s *Service) GetUserPolicies(userID string) map[string]any {
	policies := DefaultPolicies()

	roles, err := s.GetUserRoles(userID)
	if err != nil || len(roles) == 0 {
		return policies
	}

	// ロールポリシーのマージ (数値はmax、boolはOR)
	// 簡略化版: ロールの policies JSON から useDefault=false のものを適用
	for _, r := range roles {
		applyRolePolicies(policies, r)
	}

	return policies
}

// applyRolePolicies merges role-specific policy overrides into the base policies.
func applyRolePolicies(base map[string]any, role *model.Role) {
	if len(role.Policies) == 0 {
		return
	}
	// role.Policies は {"key": {"useDefault": bool, "priority": int, "value": any}} 形式
	var rolePolicies map[string]struct {
		UseDefault bool `json:"useDefault"`
		Priority   int  `json:"priority"`
		Value      any  `json:"value"`
	}
	if err := json.Unmarshal(role.Policies, &rolePolicies); err != nil {
		return
	}
	for key, p := range rolePolicies {
		if p.UseDefault {
			continue
		}
		base[key] = p.Value
	}
}

// Assign assigns a role to a user with an optional expiration.
func (s *Service) Assign(userID, roleID string, expiresAt *time.Time) error {
	if _, err := s.roleRepo.FindByID(roleID); err != nil {
		return ErrRoleNotFound
	}
	exists, err := s.assignmentRepo.Exists(userID, roleID)
	if err != nil {
		return err
	}
	if exists {
		return ErrAlreadyAssigned
	}
	a := &model.RoleAssignment{
		ID:        s.idGen.Generate(time.Now()),
		UserID:    userID,
		RoleID:    roleID,
		ExpiresAt: expiresAt,
	}
	return s.assignmentRepo.Create(a)
}

// Unassign removes a role from a user.
func (s *Service) Unassign(userID, roleID string) error {
	exists, err := s.assignmentRepo.Exists(userID, roleID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotAssigned
	}
	return s.assignmentRepo.Delete(userID, roleID)
}

// Create creates a new role.
func (s *Service) Create(name, description string, opts CreateOptions) (*model.Role, error) {
	now := time.Now()
	role := &model.Role{
		ID:              s.idGen.Generate(now),
		UpdatedAt:       now,
		LastUsedAt:      now,
		Name:            name,
		Description:     description,
		IsModerator:     opts.IsModerator,
		IsAdministrator: opts.IsAdministrator,
		IsPublic:        opts.IsPublic,
		AsBadge:         opts.AsBadge,
		IsExplorable:    opts.IsExplorable,
		Target:          model.RoleTargetManual,
		DisplayOrder:    opts.DisplayOrder,
	}
	if err := s.roleRepo.Create(role); err != nil {
		return nil, err
	}
	return role, nil
}

// CreateOptions holds optional parameters for role creation.
type CreateOptions struct {
	IsModerator     bool
	IsAdministrator bool
	IsPublic        bool
	AsBadge         bool
	IsExplorable    bool
	DisplayOrder    int
}

// Show returns a role by ID.
func (s *Service) Show(id string) (*model.Role, error) {
	r, err := s.roleRepo.FindByID(id)
	if err != nil {
		return nil, ErrRoleNotFound
	}
	return r, nil
}

// List returns all roles.
func (s *Service) List() ([]*model.Role, error) {
	return s.roleRepo.List()
}

// Delete removes a role.
func (s *Service) Delete(id string) error {
	if _, err := s.roleRepo.FindByID(id); err != nil {
		return ErrRoleNotFound
	}
	return s.roleRepo.Delete(id)
}

// DefaultPolicies returns the Misskey default policies.
func DefaultPolicies() map[string]any {
	return map[string]any{
		"gtlAvailable":               true,
		"ltlAvailable":               true,
		"canPublicNote":              true,
		"mentionLimit":               20,
		"canInvite":                  false,
		"inviteLimit":                0,
		"inviteLimitCycle":           10080,
		"inviteExpirationTime":       0,
		"canManageCustomEmojis":      false,
		"canManageAvatarDecorations": false,
		"canSearchNotes":             false,
		"canSearchUsers":             true,
		"canUseTranslator":           true,
		"canHideAds":                 false,
		"driveCapacityMb":            100,
		"maxFileSizeMb":              30,
		"alwaysMarkNsfw":             false,
		"canUpdateBioMedia":          true,
		"pinLimit":                   5,
		"antennaLimit":               5,
		"wordMuteLimit":              200,
		"webhookLimit":               3,
		"clipLimit":                  10,
		"noteEachClipsLimit":         200,
		"userListLimit":              10,
		"userEachUserListsLimit":     50,
		"rateLimitFactor":            1,
		"avatarDecorationLimit":      1,
		"canImportAntennas":          false,
		"canImportBlocking":          false,
		"canImportFollowing":         false,
		"canImportMuting":            false,
		"canImportUserLists":         false,
		"chatAvailability":           "available",
		"uploadableFileTypes":        []string{"text/*", "application/json", "image/*", "video/*", "audio/*"},
		"noteDraftLimit":             10,
		"scheduledNoteLimit":         1,
		"watermarkAvailable":         true,
	}
}
