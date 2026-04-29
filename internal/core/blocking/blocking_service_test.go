package blocking_test

import (
	"errors"
	"testing"

	"github.com/shiroha-a/mk/internal/core/blocking"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var stubError = errors.New("stub error")

func newSvc(t *testing.T) (*blocking.Service, *testutil.MockUserRepository, *testutil.MockBlockingRepository, *testutil.MockFollowingRepository) {
	t.Helper()
	userRepo := testutil.NewMockUserRepository()
	blockingRepo := testutil.NewMockBlockingRepository()
	followingRepo := testutil.NewMockFollowingRepository()
	idGen, _ := id.NewGenerator("aidx")
	svc := blocking.NewService(userRepo, blockingRepo, followingRepo, idGen)
	return svc, userRepo, blockingRepo, followingRepo
}

func addUser(repo *testutil.MockUserRepository, id string) {
	repo.Users[id] = &model.User{ID: id, Username: id}
}

func TestBlock_Self(t *testing.T) {
	svc, _, _, _ := newSvc(t)
	_, err := svc.Block("a", "a")
	require.ErrorIs(t, err, blocking.ErrSelfBlock)
}

func TestBlock_NotFound(t *testing.T) {
	svc, _, _, _ := newSvc(t)
	_, err := svc.Block("a", "b")
	require.ErrorIs(t, err, blocking.ErrBlockeeNotFound)
}

func TestBlock_AlreadyBlocking(t *testing.T) {
	svc, ur, _, _ := newSvc(t)
	addUser(ur, "a")
	addUser(ur, "b")
	_, err := svc.Block("a", "b")
	require.NoError(t, err)
	_, err = svc.Block("a", "b")
	require.ErrorIs(t, err, blocking.ErrAlreadyBlocking)
}

func TestBlock_Success(t *testing.T) {
	svc, ur, _, _ := newSvc(t)
	addUser(ur, "a")
	addUser(ur, "b")
	b, err := svc.Block("a", "b")
	require.NoError(t, err)
	assert.Equal(t, "a", b.BlockerID)
}

func TestBlock_RemovesExistingFollows(t *testing.T) {
	svc, ur, _, fr := newSvc(t)
	addUser(ur, "a")
	addUser(ur, "b")
	fr.Followings["f1"] = &model.Following{ID: "f1", FollowerID: "a", FolloweeID: "b"}
	fr.Followings["f2"] = &model.Following{ID: "f2", FollowerID: "b", FolloweeID: "a"}

	_, err := svc.Block("a", "b")
	require.NoError(t, err)
	assert.Empty(t, fr.Followings)
}

func TestBlock_NoFollowingRepo(t *testing.T) {
	userRepo := testutil.NewMockUserRepository()
	addUser(userRepo, "a")
	addUser(userRepo, "b")
	idGen, _ := id.NewGenerator("aidx")
	svc := blocking.NewService(userRepo, testutil.NewMockBlockingRepository(), nil, idGen)
	_, err := svc.Block("a", "b")
	require.NoError(t, err)
}

// failingBlockingRepo wraps mock to fail Exists/Create.
type failingBlockingRepo struct {
	*testutil.MockBlockingRepository
	failExists bool
	failCreate bool
}

func (f *failingBlockingRepo) Exists(blockerID, blockeeID string) (bool, error) {
	if f.failExists {
		return false, stubError
	}
	return f.MockBlockingRepository.Exists(blockerID, blockeeID)
}

func (f *failingBlockingRepo) Create(b *model.Blocking) error {
	if f.failCreate {
		return stubError
	}
	return f.MockBlockingRepository.Create(b)
}

func TestBlock_ExistsError(t *testing.T) {
	userRepo := testutil.NewMockUserRepository()
	addUser(userRepo, "a")
	addUser(userRepo, "b")
	idGen, _ := id.NewGenerator("aidx")
	svc := blocking.NewService(userRepo, &failingBlockingRepo{MockBlockingRepository: testutil.NewMockBlockingRepository(), failExists: true}, nil, idGen)
	_, err := svc.Block("a", "b")
	assert.ErrorIs(t, err, stubError)
}

func TestBlock_CreateError(t *testing.T) {
	userRepo := testutil.NewMockUserRepository()
	addUser(userRepo, "a")
	addUser(userRepo, "b")
	idGen, _ := id.NewGenerator("aidx")
	svc := blocking.NewService(userRepo, &failingBlockingRepo{MockBlockingRepository: testutil.NewMockBlockingRepository(), failCreate: true}, nil, idGen)
	_, err := svc.Block("a", "b")
	assert.ErrorIs(t, err, stubError)
}

func TestUnblock_Self(t *testing.T) {
	svc, _, _, _ := newSvc(t)
	err := svc.Unblock("a", "a")
	require.ErrorIs(t, err, blocking.ErrSelfBlock)
}

func TestUnblock_NotBlocking(t *testing.T) {
	svc, _, _, _ := newSvc(t)
	err := svc.Unblock("a", "b")
	require.ErrorIs(t, err, blocking.ErrNotBlocking)
}

func TestUnblock_Success(t *testing.T) {
	svc, ur, _, _ := newSvc(t)
	addUser(ur, "a")
	addUser(ur, "b")
	_, err := svc.Block("a", "b")
	require.NoError(t, err)
	require.NoError(t, svc.Unblock("a", "b"))
}

func TestIsBlockedAndList(t *testing.T) {
	svc, ur, _, _ := newSvc(t)
	addUser(ur, "a")
	addUser(ur, "b")
	_, err := svc.Block("a", "b")
	require.NoError(t, err)

	yes, err := svc.IsBlocked("a", "b")
	require.NoError(t, err)
	assert.True(t, yes)

	rows, err := svc.List("a", "", "", 0, 0)
	require.NoError(t, err)
	assert.Len(t, rows, 1)

	rows, err = svc.List("a", "", "", 5, 0)
	require.NoError(t, err)
	assert.Len(t, rows, 1)
}

// failingFollowingRepo wraps mock to make Delete fail (covers fold-in error path)
type failingFollowingRepo struct {
	*testutil.MockFollowingRepository
}

func (f *failingFollowingRepo) Delete(_ *model.Following) error {
	return stubError
}

func TestBlock_RemoveFollowingDeleteError(t *testing.T) {
	userRepo := testutil.NewMockUserRepository()
	addUser(userRepo, "a")
	addUser(userRepo, "b")
	mock := testutil.NewMockFollowingRepository()
	mock.Followings["f1"] = &model.Following{ID: "f1", FollowerID: "a", FolloweeID: "b"}
	idGen, _ := id.NewGenerator("aidx")
	svc := blocking.NewService(userRepo, testutil.NewMockBlockingRepository(),
		&failingFollowingRepo{MockFollowingRepository: mock}, idGen)
	_, err := svc.Block("a", "b")
	require.NoError(t, err)
}
