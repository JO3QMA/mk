package following_test

import (
	"errors"
	"testing"

	"github.com/shiroha-a/mk/internal/core/following"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubError is a sentinel error used to simulate repository failures.
var stubError = errors.New("stub repo error")

// failingUserRepo wraps MockUserRepository and lets each method optionally fail.
type failingUserRepo struct {
	*testutil.MockUserRepository
	failIncrementFollowing bool
	failIncrementFollowers bool
}

func (f *failingUserRepo) IncrementFollowingCount(userID string, delta int) error {
	if f.failIncrementFollowing {
		return stubError
	}
	return f.MockUserRepository.IncrementFollowingCount(userID, delta)
}

func (f *failingUserRepo) IncrementFollowersCount(userID string, delta int) error {
	if f.failIncrementFollowers {
		return stubError
	}
	return f.MockUserRepository.IncrementFollowersCount(userID, delta)
}

// failingFollowingRepo wraps MockFollowingRepository and lets each method optionally fail.
type failingFollowingRepo struct {
	*testutil.MockFollowingRepository
	failExists bool
	failCreate bool
	failDelete bool
}

func (f *failingFollowingRepo) Exists(followerID, followeeID string) (bool, error) {
	if f.failExists {
		return false, stubError
	}
	return f.MockFollowingRepository.Exists(followerID, followeeID)
}

func (f *failingFollowingRepo) Create(x *model.Following) error {
	if f.failCreate {
		return stubError
	}
	return f.MockFollowingRepository.Create(x)
}

func (f *failingFollowingRepo) Delete(x *model.Following) error {
	if f.failDelete {
		return stubError
	}
	return f.MockFollowingRepository.Delete(x)
}

// failingFollowRequestRepo wraps MockFollowRequestRepository and lets each method optionally fail.
type failingFollowRequestRepo struct {
	*testutil.MockFollowRequestRepository
	failExists bool
	failCreate bool
	failDelete bool
}

func (f *failingFollowRequestRepo) Exists(followerID, followeeID string) (bool, error) {
	if f.failExists {
		return false, stubError
	}
	return f.MockFollowRequestRepository.Exists(followerID, followeeID)
}

func (f *failingFollowRequestRepo) Create(r *model.FollowRequest) error {
	if f.failCreate {
		return stubError
	}
	return f.MockFollowRequestRepository.Create(r)
}

func (f *failingFollowRequestRepo) Delete(r *model.FollowRequest) error {
	if f.failDelete {
		return stubError
	}
	return f.MockFollowRequestRepository.Delete(r)
}

// newSvcWith builds a Service from explicit repositories.
func newSvcWith(userRepo repository.UserRepository, fRepo repository.FollowingRepository, frRepo repository.FollowRequestRepository) *following.Service {
	idGen, _ := id.NewGenerator("aidx")
	return following.NewService(userRepo, fRepo, frRepo, idGen)
}

func newSvc(t *testing.T) (*following.Service, *testutil.MockUserRepository, *testutil.MockFollowingRepository, *testutil.MockFollowRequestRepository) {
	t.Helper()
	userRepo := testutil.NewMockUserRepository()
	fRepo := testutil.NewMockFollowingRepository()
	frRepo := testutil.NewMockFollowRequestRepository()
	idGen, _ := id.NewGenerator("aidx")
	svc := following.NewService(userRepo, fRepo, frRepo, idGen)
	return svc, userRepo, fRepo, frRepo
}

func addUser(t *testing.T, repo *testutil.MockUserRepository, id string, locked bool) *model.User {
	t.Helper()
	u := &model.User{ID: id, Username: id, IsLocked: locked}
	repo.Users[id] = u
	return u
}

func TestFollow_Success(t *testing.T) {
	svc, userRepo, fRepo, _ := newSvc(t)
	addUser(t, userRepo, "alice", false)
	addUser(t, userRepo, "bob", false)

	res, err := svc.Follow("alice", "bob")
	require.NoError(t, err)
	require.NotNil(t, res.Following)
	assert.Nil(t, res.Request)
	assert.Len(t, fRepo.Followings, 1)
	assert.Equal(t, 1, userRepo.Users["alice"].FollowingCount)
	assert.Equal(t, 1, userRepo.Users["bob"].FollowersCount)
}

func TestFollow_LockedUser_CreatesRequest(t *testing.T) {
	svc, userRepo, fRepo, frRepo := newSvc(t)
	addUser(t, userRepo, "alice", false)
	addUser(t, userRepo, "bob", true)

	res, err := svc.Follow("alice", "bob")
	require.NoError(t, err)
	assert.Nil(t, res.Following)
	require.NotNil(t, res.Request)
	assert.Empty(t, fRepo.Followings)
	assert.Len(t, frRepo.Requests, 1)
	// counterは更新されない
	assert.Equal(t, 0, userRepo.Users["alice"].FollowingCount)
	assert.Equal(t, 0, userRepo.Users["bob"].FollowersCount)
}

func TestFollow_SelfFollow(t *testing.T) {
	svc, userRepo, _, _ := newSvc(t)
	addUser(t, userRepo, "alice", false)

	_, err := svc.Follow("alice", "alice")
	assert.True(t, errors.Is(err, following.ErrSelfFollow))
}

func TestFollow_FolloweeNotFound(t *testing.T) {
	svc, userRepo, _, _ := newSvc(t)
	addUser(t, userRepo, "alice", false)

	_, err := svc.Follow("alice", "missing")
	assert.True(t, errors.Is(err, following.ErrFolloweeNotFound))
}

func TestFollow_FollowerNotFound(t *testing.T) {
	svc, userRepo, _, _ := newSvc(t)
	addUser(t, userRepo, "bob", false)

	_, err := svc.Follow("missing", "bob")
	assert.True(t, errors.Is(err, following.ErrFolloweeNotFound))
}

func TestFollow_AlreadyFollowing(t *testing.T) {
	svc, userRepo, _, _ := newSvc(t)
	addUser(t, userRepo, "alice", false)
	addUser(t, userRepo, "bob", false)

	_, err := svc.Follow("alice", "bob")
	require.NoError(t, err)
	_, err = svc.Follow("alice", "bob")
	assert.True(t, errors.Is(err, following.ErrAlreadyFollowing))
}

func TestFollow_AlreadyRequested(t *testing.T) {
	svc, userRepo, _, _ := newSvc(t)
	addUser(t, userRepo, "alice", false)
	addUser(t, userRepo, "bob", true)

	_, err := svc.Follow("alice", "bob")
	require.NoError(t, err)
	_, err = svc.Follow("alice", "bob")
	assert.True(t, errors.Is(err, following.ErrAlreadyRequested))
}

func TestUnfollow_Success(t *testing.T) {
	svc, userRepo, fRepo, _ := newSvc(t)
	addUser(t, userRepo, "alice", false)
	addUser(t, userRepo, "bob", false)
	_, err := svc.Follow("alice", "bob")
	require.NoError(t, err)

	err = svc.Unfollow("alice", "bob")
	require.NoError(t, err)
	assert.Empty(t, fRepo.Followings)
	assert.Equal(t, 0, userRepo.Users["alice"].FollowingCount)
	assert.Equal(t, 0, userRepo.Users["bob"].FollowersCount)
}

func TestUnfollow_NotFollowing(t *testing.T) {
	svc, userRepo, _, _ := newSvc(t)
	addUser(t, userRepo, "alice", false)
	addUser(t, userRepo, "bob", false)

	err := svc.Unfollow("alice", "bob")
	assert.True(t, errors.Is(err, following.ErrNotFollowing))
}

func TestUnfollow_SelfFollow(t *testing.T) {
	svc, _, _, _ := newSvc(t)
	err := svc.Unfollow("alice", "alice")
	assert.True(t, errors.Is(err, following.ErrSelfFollow))
}

func TestAcceptRequest_Success(t *testing.T) {
	svc, userRepo, fRepo, frRepo := newSvc(t)
	addUser(t, userRepo, "alice", false)
	addUser(t, userRepo, "bob", true)
	_, err := svc.Follow("alice", "bob")
	require.NoError(t, err)

	err = svc.AcceptRequest("bob", "alice")
	require.NoError(t, err)
	assert.Empty(t, frRepo.Requests)
	assert.Len(t, fRepo.Followings, 1)
	assert.Equal(t, 1, userRepo.Users["alice"].FollowingCount)
	assert.Equal(t, 1, userRepo.Users["bob"].FollowersCount)
}

func TestAcceptRequest_NotFound(t *testing.T) {
	svc, _, _, _ := newSvc(t)
	err := svc.AcceptRequest("bob", "alice")
	assert.True(t, errors.Is(err, following.ErrRequestNotFound))
}

func TestRejectRequest_Success(t *testing.T) {
	svc, userRepo, fRepo, frRepo := newSvc(t)
	addUser(t, userRepo, "alice", false)
	addUser(t, userRepo, "bob", true)
	_, err := svc.Follow("alice", "bob")
	require.NoError(t, err)

	err = svc.RejectRequest("bob", "alice")
	require.NoError(t, err)
	assert.Empty(t, frRepo.Requests)
	assert.Empty(t, fRepo.Followings)
}

func TestRejectRequest_NotFound(t *testing.T) {
	svc, _, _, _ := newSvc(t)
	err := svc.RejectRequest("bob", "alice")
	assert.True(t, errors.Is(err, following.ErrRequestNotFound))
}

func TestCancelRequest_Success(t *testing.T) {
	svc, userRepo, _, frRepo := newSvc(t)
	addUser(t, userRepo, "alice", false)
	addUser(t, userRepo, "bob", true)
	_, err := svc.Follow("alice", "bob")
	require.NoError(t, err)

	err = svc.CancelRequest("alice", "bob")
	require.NoError(t, err)
	assert.Empty(t, frRepo.Requests)
}

func TestCancelRequest_NotFound(t *testing.T) {
	svc, _, _, _ := newSvc(t)
	err := svc.CancelRequest("alice", "bob")
	assert.True(t, errors.Is(err, following.ErrRequestNotFound))
}

func TestListReceivedRequests(t *testing.T) {
	svc, userRepo, _, _ := newSvc(t)
	addUser(t, userRepo, "alice", false)
	addUser(t, userRepo, "carol", false)
	addUser(t, userRepo, "bob", true)
	_, _ = svc.Follow("alice", "bob")
	_, _ = svc.Follow("carol", "bob")

	rows, err := svc.ListReceivedRequests("bob", 100, 0)
	require.NoError(t, err)
	assert.Len(t, rows, 2)
}

func TestListReceivedFollowing(t *testing.T) {
	svc, userRepo, _, _ := newSvc(t)
	addUser(t, userRepo, "alice", false)
	addUser(t, userRepo, "bob", false)
	addUser(t, userRepo, "carol", false)
	_, _ = svc.Follow("bob", "alice")
	_, _ = svc.Follow("carol", "alice")

	rows, err := svc.ListReceivedFollowing("alice", 10, 0)
	require.NoError(t, err)
	assert.Len(t, rows, 2)
}

func TestListSentFollowing(t *testing.T) {
	svc, userRepo, _, _ := newSvc(t)
	addUser(t, userRepo, "alice", false)
	addUser(t, userRepo, "bob", false)
	addUser(t, userRepo, "carol", false)
	_, _ = svc.Follow("alice", "bob")
	_, _ = svc.Follow("alice", "carol")

	rows, err := svc.ListSentFollowing("alice", 10, 0)
	require.NoError(t, err)
	assert.Len(t, rows, 2)
}

func TestListSentRequests(t *testing.T) {
	svc, userRepo, _, _ := newSvc(t)
	addUser(t, userRepo, "alice", false)
	addUser(t, userRepo, "bob", true)
	addUser(t, userRepo, "dave", true)
	_, _ = svc.Follow("alice", "bob")
	_, _ = svc.Follow("alice", "dave")

	rows, err := svc.ListSentRequests("alice", 100, 0)
	require.NoError(t, err)
	assert.Len(t, rows, 2)
}

// --- failing-repo error path tests ---

func setupUsers(repo *testutil.MockUserRepository, ids ...string) {
	for _, i := range ids {
		repo.Users[i] = &model.User{ID: i, Username: i}
	}
}

func setupLockedUser(repo *testutil.MockUserRepository, id string) {
	repo.Users[id] = &model.User{ID: id, Username: id, IsLocked: true}
}

func TestFollow_ExistsError(t *testing.T) {
	userRepo := testutil.NewMockUserRepository()
	setupUsers(userRepo, "alice", "bob")
	fRepo := &failingFollowingRepo{MockFollowingRepository: testutil.NewMockFollowingRepository(), failExists: true}
	frRepo := testutil.NewMockFollowRequestRepository()
	svc := newSvcWith(userRepo, fRepo, frRepo)

	_, err := svc.Follow("alice", "bob")
	assert.ErrorIs(t, err, stubError)
}

func TestFollow_CreateError(t *testing.T) {
	userRepo := testutil.NewMockUserRepository()
	setupUsers(userRepo, "alice", "bob")
	fRepo := &failingFollowingRepo{MockFollowingRepository: testutil.NewMockFollowingRepository(), failCreate: true}
	frRepo := testutil.NewMockFollowRequestRepository()
	svc := newSvcWith(userRepo, fRepo, frRepo)

	_, err := svc.Follow("alice", "bob")
	assert.ErrorIs(t, err, stubError)
}

func TestFollow_RequestExistsError(t *testing.T) {
	userRepo := testutil.NewMockUserRepository()
	setupUsers(userRepo, "alice")
	setupLockedUser(userRepo, "bob")
	fRepo := testutil.NewMockFollowingRepository()
	frRepo := &failingFollowRequestRepo{MockFollowRequestRepository: testutil.NewMockFollowRequestRepository(), failExists: true}
	svc := newSvcWith(userRepo, fRepo, frRepo)

	_, err := svc.Follow("alice", "bob")
	assert.ErrorIs(t, err, stubError)
}

func TestFollow_RequestCreateError(t *testing.T) {
	userRepo := testutil.NewMockUserRepository()
	setupUsers(userRepo, "alice")
	setupLockedUser(userRepo, "bob")
	fRepo := testutil.NewMockFollowingRepository()
	frRepo := &failingFollowRequestRepo{MockFollowRequestRepository: testutil.NewMockFollowRequestRepository(), failCreate: true}
	svc := newSvcWith(userRepo, fRepo, frRepo)

	_, err := svc.Follow("alice", "bob")
	assert.ErrorIs(t, err, stubError)
}

func TestFollow_IncrementFollowingError(t *testing.T) {
	mockUR := testutil.NewMockUserRepository()
	setupUsers(mockUR, "alice", "bob")
	userRepo := &failingUserRepo{MockUserRepository: mockUR, failIncrementFollowing: true}
	fRepo := testutil.NewMockFollowingRepository()
	frRepo := testutil.NewMockFollowRequestRepository()
	svc := newSvcWith(userRepo, fRepo, frRepo)

	_, err := svc.Follow("alice", "bob")
	assert.ErrorIs(t, err, stubError)
}

func TestFollow_IncrementFollowersError(t *testing.T) {
	mockUR := testutil.NewMockUserRepository()
	setupUsers(mockUR, "alice", "bob")
	userRepo := &failingUserRepo{MockUserRepository: mockUR, failIncrementFollowers: true}
	fRepo := testutil.NewMockFollowingRepository()
	frRepo := testutil.NewMockFollowRequestRepository()
	svc := newSvcWith(userRepo, fRepo, frRepo)

	_, err := svc.Follow("alice", "bob")
	assert.ErrorIs(t, err, stubError)
}

func TestUnfollow_DeleteError(t *testing.T) {
	mockUR := testutil.NewMockUserRepository()
	setupUsers(mockUR, "alice", "bob")
	mockFR := testutil.NewMockFollowingRepository()
	// 既存のfollowingを直接挿入
	mockFR.Followings["existing"] = &model.Following{ID: "existing", FollowerID: "alice", FolloweeID: "bob"}
	fRepo := &failingFollowingRepo{MockFollowingRepository: mockFR, failDelete: true}
	frRepo := testutil.NewMockFollowRequestRepository()
	svc := newSvcWith(mockUR, fRepo, frRepo)

	err := svc.Unfollow("alice", "bob")
	assert.ErrorIs(t, err, stubError)
}

func TestUnfollow_IncrementFollowingError(t *testing.T) {
	mockUR := testutil.NewMockUserRepository()
	setupUsers(mockUR, "alice", "bob")
	userRepo := &failingUserRepo{MockUserRepository: mockUR, failIncrementFollowing: true}
	mockFR := testutil.NewMockFollowingRepository()
	mockFR.Followings["existing"] = &model.Following{ID: "existing", FollowerID: "alice", FolloweeID: "bob"}
	frRepo := testutil.NewMockFollowRequestRepository()
	svc := newSvcWith(userRepo, mockFR, frRepo)

	err := svc.Unfollow("alice", "bob")
	assert.ErrorIs(t, err, stubError)
}

func TestUnfollow_IncrementFollowersError(t *testing.T) {
	mockUR := testutil.NewMockUserRepository()
	setupUsers(mockUR, "alice", "bob")
	userRepo := &failingUserRepo{MockUserRepository: mockUR, failIncrementFollowers: true}
	mockFR := testutil.NewMockFollowingRepository()
	mockFR.Followings["existing"] = &model.Following{ID: "existing", FollowerID: "alice", FolloweeID: "bob"}
	frRepo := testutil.NewMockFollowRequestRepository()
	svc := newSvcWith(userRepo, mockFR, frRepo)

	err := svc.Unfollow("alice", "bob")
	assert.ErrorIs(t, err, stubError)
}

func TestAcceptRequest_DeleteError(t *testing.T) {
	mockUR := testutil.NewMockUserRepository()
	setupUsers(mockUR, "alice", "bob")
	mockFRR := testutil.NewMockFollowRequestRepository()
	mockFRR.Requests["req"] = &model.FollowRequest{ID: "req", FollowerID: "alice", FolloweeID: "bob"}
	frRepo := &failingFollowRequestRepo{MockFollowRequestRepository: mockFRR, failDelete: true}
	svc := newSvcWith(mockUR, testutil.NewMockFollowingRepository(), frRepo)

	err := svc.AcceptRequest("bob", "alice")
	assert.ErrorIs(t, err, stubError)
}

func TestAcceptRequest_FollowingCreateError(t *testing.T) {
	mockUR := testutil.NewMockUserRepository()
	setupUsers(mockUR, "alice", "bob")
	mockFRR := testutil.NewMockFollowRequestRepository()
	mockFRR.Requests["req"] = &model.FollowRequest{ID: "req", FollowerID: "alice", FolloweeID: "bob"}
	mockFR := testutil.NewMockFollowingRepository()
	fRepo := &failingFollowingRepo{MockFollowingRepository: mockFR, failCreate: true}
	svc := newSvcWith(mockUR, fRepo, mockFRR)

	err := svc.AcceptRequest("bob", "alice")
	assert.ErrorIs(t, err, stubError)
}

func TestAcceptRequest_IncrementFollowingError(t *testing.T) {
	mockUR := testutil.NewMockUserRepository()
	setupUsers(mockUR, "alice", "bob")
	userRepo := &failingUserRepo{MockUserRepository: mockUR, failIncrementFollowing: true}
	mockFRR := testutil.NewMockFollowRequestRepository()
	mockFRR.Requests["req"] = &model.FollowRequest{ID: "req", FollowerID: "alice", FolloweeID: "bob"}
	svc := newSvcWith(userRepo, testutil.NewMockFollowingRepository(), mockFRR)

	err := svc.AcceptRequest("bob", "alice")
	assert.ErrorIs(t, err, stubError)
}

func TestAcceptRequest_IncrementFollowersError(t *testing.T) {
	mockUR := testutil.NewMockUserRepository()
	setupUsers(mockUR, "alice", "bob")
	userRepo := &failingUserRepo{MockUserRepository: mockUR, failIncrementFollowers: true}
	mockFRR := testutil.NewMockFollowRequestRepository()
	mockFRR.Requests["req"] = &model.FollowRequest{ID: "req", FollowerID: "alice", FolloweeID: "bob"}
	svc := newSvcWith(userRepo, testutil.NewMockFollowingRepository(), mockFRR)

	err := svc.AcceptRequest("bob", "alice")
	assert.ErrorIs(t, err, stubError)
}
