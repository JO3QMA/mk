package signup_test

import (
	"testing"

	"github.com/shiroha-a/mk/internal/core/signup"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestService(t *testing.T) (*signup.Service, *testutil.MockUserRepository, *testutil.MockMetaRepository) {
	t.Helper()
	userRepo := testutil.NewMockUserRepository()
	metaRepo := testutil.NewMockMetaRepository()
	metaRepo.Meta = &model.Meta{ID: "x"}
	idGen, _ := id.NewGenerator("aidx")
	svc := signup.NewService(userRepo, metaRepo, idGen)
	return svc, userRepo, metaRepo
}

func TestSignup_Success(t *testing.T) {
	svc, userRepo, _ := newTestService(t)
	result, err := svc.Signup("testuser", "password123", false)
	require.NoError(t, err)
	assert.Equal(t, "testuser", result.User.Username)
	assert.NotEmpty(t, result.Token)
	assert.Len(t, userRepo.Users, 1)
	assert.Len(t, userRepo.Profiles, 1)

	// パスワードがハッシュ化されている
	profile := userRepo.Profiles[result.User.ID]
	assert.NotNil(t, profile.Password)
	assert.NotEqual(t, "password123", *profile.Password)
}

func TestSignup_InitialSetup_SetsRootUser(t *testing.T) {
	svc, _, metaRepo := newTestService(t)
	result, err := svc.Signup("admin", "pass", true)
	require.NoError(t, err)

	// rootUserId が設定される
	assert.NotNil(t, metaRepo.Meta.RootUserID)
	assert.Equal(t, result.User.ID, *metaRepo.Meta.RootUserID)
}

func TestSignup_NotInitialSetup_DoesNotSetRootUser(t *testing.T) {
	svc, _, metaRepo := newTestService(t)
	_, err := svc.Signup("user1", "pass", false)
	require.NoError(t, err)
	assert.Nil(t, metaRepo.Meta.RootUserID)
}

func TestSignup_DuplicateUsername(t *testing.T) {
	svc, userRepo, _ := newTestService(t)
	userRepo.Users["existing"] = &model.User{
		ID:            "existing",
		Username:      "taken",
		UsernameLower: "taken",
	}

	_, err := svc.Signup("taken", "pass", false)
	assert.ErrorIs(t, err, signup.ErrUsernameAlreadyExists)
}

func TestSignup_EmptyUsername(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.Signup("", "pass", false)
	assert.ErrorIs(t, err, signup.ErrInvalidUsername)
}

func TestSignup_TooLongUsername(t *testing.T) {
	svc, _, _ := newTestService(t)
	long := make([]byte, 200)
	for i := range long {
		long[i] = 'a'
	}
	_, err := svc.Signup(string(long), "pass", false)
	assert.ErrorIs(t, err, signup.ErrInvalidUsername)
}

// --- Failing repo tests ---

type failingCreateUserRepo struct {
	*testutil.MockUserRepository
}

func (f *failingCreateUserRepo) Create(_ *model.User) error { return assert.AnError }

type failingCreateProfileRepo struct {
	*testutil.MockUserRepository
}

func (f *failingCreateProfileRepo) CreateProfile(_ *model.UserProfile) error { return assert.AnError }

func TestSignup_UserCreateError(t *testing.T) {
	repo := &failingCreateUserRepo{testutil.NewMockUserRepository()}
	metaRepo := testutil.NewMockMetaRepository()
	metaRepo.Meta = &model.Meta{ID: "x"}
	idGen, _ := id.NewGenerator("aidx")
	svc := signup.NewService(repo, metaRepo, idGen)
	_, err := svc.Signup("user1", "pass", false)
	assert.Error(t, err)
}

func TestSignup_ProfileCreateError(t *testing.T) {
	repo := &failingCreateProfileRepo{testutil.NewMockUserRepository()}
	metaRepo := testutil.NewMockMetaRepository()
	metaRepo.Meta = &model.Meta{ID: "x"}
	idGen, _ := id.NewGenerator("aidx")
	svc := signup.NewService(repo, metaRepo, idGen)
	_, err := svc.Signup("user1", "pass", false)
	assert.Error(t, err)
}

func TestSignup_TokenIs16Chars(t *testing.T) {
	svc, _, _ := newTestService(t)
	result, err := svc.Signup("user1", "pass", false)
	require.NoError(t, err)
	assert.Len(t, result.Token, 16) // 8 bytes hex = 16 chars
}

func TestSignup_WithKeypairRepo(t *testing.T) {
	svc, _, _ := newTestService(t)
	keypairRepo := testutil.NewMockUserKeypairRepository()
	svc.SetKeypairRepo(keypairRepo)

	result, err := svc.Signup("alice", "pass", false)
	require.NoError(t, err)

	// Keypair created for the new user.
	k, ok := keypairRepo.Keypairs[result.User.ID]
	require.True(t, ok)
	assert.NotEmpty(t, k.PublicKey)
	assert.NotEmpty(t, k.PrivateKey)
}

type failingCreateKeypairRepo struct{}

func (f *failingCreateKeypairRepo) Create(_ *model.UserKeypair) error { return assert.AnError }
func (f *failingCreateKeypairRepo) FindByUserID(_ string) (*model.UserKeypair, error) {
	return nil, assert.AnError
}

func TestSignup_KeypairCreateError(t *testing.T) {
	svc, _, _ := newTestService(t)
	svc.SetKeypairRepo(&failingCreateKeypairRepo{})

	_, err := svc.Signup("alice", "pass", false)
	assert.Error(t, err)
}
