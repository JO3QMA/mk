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

// recordingChartHook captures Follow / Unfollow events forwarded by
// FollowingService so we can verify the chart hook firing path.
type recordingChartHook struct {
	follows   [][2]string
	unfollows [][2]string
}

func (h *recordingChartHook) OnFollow(follower, followee *model.User) {
	h.follows = append(h.follows, [2]string{follower.ID, followee.ID})
}

func (h *recordingChartHook) OnUnfollow(follower, followee *model.User) {
	h.unfollows = append(h.unfollows, [2]string{follower.ID, followee.ID})
}

func TestFollow_ChartHookInvoked(t *testing.T) {
	svc, userRepo, _, _ := newSvc(t)
	addUser(t, userRepo, "alice", false)
	addUser(t, userRepo, "bob", false)
	hook := &recordingChartHook{}
	svc.SetChartHook(hook)

	_, err := svc.Follow("alice", "bob")
	require.NoError(t, err)
	require.Len(t, hook.follows, 1)
	assert.Equal(t, [2]string{"alice", "bob"}, hook.follows[0])
}

func TestUnfollow_ChartHookInvoked(t *testing.T) {
	svc, userRepo, _, _ := newSvc(t)
	addUser(t, userRepo, "alice", false)
	addUser(t, userRepo, "bob", false)
	_, err := svc.Follow("alice", "bob")
	require.NoError(t, err)

	hook := &recordingChartHook{}
	svc.SetChartHook(hook)

	require.NoError(t, svc.Unfollow("alice", "bob"))
	require.Len(t, hook.unfollows, 1)
	assert.Equal(t, [2]string{"alice", "bob"}, hook.unfollows[0])
}

func TestAcceptRequest_ChartHookInvoked(t *testing.T) {
	svc, userRepo, _, _ := newSvc(t)
	addUser(t, userRepo, "alice", false)
	addUser(t, userRepo, "bob", true)
	_, err := svc.Follow("alice", "bob")
	require.NoError(t, err)

	hook := &recordingChartHook{}
	svc.SetChartHook(hook)

	require.NoError(t, svc.AcceptRequest("bob", "alice"))
	require.Len(t, hook.follows, 1)
	assert.Equal(t, [2]string{"alice", "bob"}, hook.follows[0])
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

// recordingHook captures notification hook calls.
type recordingHook struct {
	follows  []string
	requests []string
	accepts  []string
}

func (h *recordingHook) OnFollowed(followerID, followeeID string) {
	h.follows = append(h.follows, followerID+"->"+followeeID)
}
func (h *recordingHook) OnFollowRequested(followerID, followeeID string) {
	h.requests = append(h.requests, followerID+"->"+followeeID)
}
func (h *recordingHook) OnFollowAccepted(followerID, followeeID string) {
	h.accepts = append(h.accepts, followerID+"->"+followeeID)
}

func TestService_NotificationHook_OnFollow(t *testing.T) {
	svc, ur, _, _ := newSvc(t)
	addUser(t, ur, "alice", false)
	addUser(t, ur, "bob", false)
	hook := &recordingHook{}
	svc.SetNotificationHook(hook)

	_, err := svc.Follow("bob", "alice")
	require.NoError(t, err)
	assert.Equal(t, []string{"bob->alice"}, hook.follows)
}

func TestService_NotificationHook_OnFollowRequest(t *testing.T) {
	svc, ur, _, _ := newSvc(t)
	addUser(t, ur, "alice", true)
	addUser(t, ur, "bob", false)
	hook := &recordingHook{}
	svc.SetNotificationHook(hook)

	_, err := svc.Follow("bob", "alice")
	require.NoError(t, err)
	assert.Equal(t, []string{"bob->alice"}, hook.requests)
}

// stubBlockingChecker is a configurable BlockingChecker for tests.
type stubBlockingChecker struct {
	blockedPairs map[string]bool // "blockerID->blockeeID"
	err          error
}

func (s *stubBlockingChecker) IsBlocked(blockerID, blockeeID string) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.blockedPairs[blockerID+"->"+blockeeID], nil
}

func TestFollow_BlockedByFollowee(t *testing.T) {
	svc, ur, _, _ := newSvc(t)
	addUser(t, ur, "alice", false)
	addUser(t, ur, "bob", false)
	svc.SetBlockingChecker(&stubBlockingChecker{blockedPairs: map[string]bool{"alice->bob": true}})

	_, err := svc.Follow("bob", "alice")
	require.ErrorIs(t, err, following.ErrBlocked)
}

func TestFollow_BlockedFollowee(t *testing.T) {
	svc, ur, _, _ := newSvc(t)
	addUser(t, ur, "alice", false)
	addUser(t, ur, "bob", false)
	svc.SetBlockingChecker(&stubBlockingChecker{blockedPairs: map[string]bool{"bob->alice": true}})

	_, err := svc.Follow("bob", "alice")
	require.ErrorIs(t, err, following.ErrBlocked)
}

func TestFollow_BlockingCheckerReverseError(t *testing.T) {
	svc, ur, _, _ := newSvc(t)
	addUser(t, ur, "alice", false)
	addUser(t, ur, "bob", false)
	svc.SetBlockingChecker(&stubBlockingChecker{err: stubError})
	_, err := svc.Follow("bob", "alice")
	assert.ErrorIs(t, err, stubError)
}

// stubBlockingCheckerForwardError errors only on forward (follower->followee) check.
type stubBlockingCheckerForwardError struct {
	calls int
}

func (s *stubBlockingCheckerForwardError) IsBlocked(_, _ string) (bool, error) {
	s.calls++
	if s.calls == 2 {
		return false, stubError
	}
	return false, nil
}

func TestFollow_BlockingCheckerForwardError(t *testing.T) {
	svc, ur, _, _ := newSvc(t)
	addUser(t, ur, "alice", false)
	addUser(t, ur, "bob", false)
	svc.SetBlockingChecker(&stubBlockingCheckerForwardError{})
	_, err := svc.Follow("bob", "alice")
	assert.ErrorIs(t, err, stubError)
}

func TestService_NotificationHook_OnAccept(t *testing.T) {
	svc, ur, _, frRepo := newSvc(t)
	addUser(t, ur, "alice", true)
	addUser(t, ur, "bob", false)
	frRepo.Requests["req"] = &model.FollowRequest{ID: "req", FollowerID: "bob", FolloweeID: "alice"}
	hook := &recordingHook{}
	svc.SetNotificationHook(hook)

	require.NoError(t, svc.AcceptRequest("alice", "bob"))
	assert.Equal(t, []string{"bob->alice"}, hook.accepts)
}

// recordingFederationHook captures federation hook invocations for assertion.
type recordingFederationHook struct {
	follows   []string
	unfollows []string
	accepts   []string
}

func (h *recordingFederationHook) OnLocalFollowed(follower, followee *model.User) {
	h.follows = append(h.follows, follower.ID+"->"+followee.ID)
}

func (h *recordingFederationHook) OnLocalUnfollowed(follower, followee *model.User) {
	h.unfollows = append(h.unfollows, follower.ID+"->"+followee.ID)
}

func (h *recordingFederationHook) OnLocalFollowAccepted(follower, followee *model.User) {
	h.accepts = append(h.accepts, follower.ID+"->"+followee.ID)
}

func TestService_FederationHook_OnFollow(t *testing.T) {
	svc, ur, _, _ := newSvc(t)
	addUser(t, ur, "alice", false)
	addUser(t, ur, "bob", false)
	hook := &recordingFederationHook{}
	svc.SetFederationHook(hook)

	_, err := svc.Follow("alice", "bob")
	require.NoError(t, err)
	assert.Equal(t, []string{"alice->bob"}, hook.follows)
}

func TestService_FederationHook_OnUnfollow(t *testing.T) {
	svc, ur, _, _ := newSvc(t)
	addUser(t, ur, "alice", false)
	addUser(t, ur, "bob", false)
	_, err := svc.Follow("alice", "bob")
	require.NoError(t, err)
	hook := &recordingFederationHook{}
	svc.SetFederationHook(hook)

	require.NoError(t, svc.Unfollow("alice", "bob"))
	assert.Equal(t, []string{"alice->bob"}, hook.unfollows)
}

func TestService_FederationHook_OnUnfollow_UserLookupFailure(t *testing.T) {
	// follower の repo から FindByID が失敗する経路: alice をフォロー後に
	// repo から削除する。
	svc, ur, _, _ := newSvc(t)
	addUser(t, ur, "alice", false)
	addUser(t, ur, "bob", false)
	_, err := svc.Follow("alice", "bob")
	require.NoError(t, err)
	hook := &recordingFederationHook{}
	svc.SetFederationHook(hook)

	delete(ur.Users, "alice")
	require.NoError(t, svc.Unfollow("alice", "bob"))
	assert.Empty(t, hook.unfollows)
}

func TestService_FederationHook_OnAccept(t *testing.T) {
	svc, ur, _, frRepo := newSvc(t)
	addUser(t, ur, "alice", true)
	addUser(t, ur, "bob", false)
	frRepo.Requests["req"] = &model.FollowRequest{ID: "req", FollowerID: "bob", FolloweeID: "alice"}
	hook := &recordingFederationHook{}
	svc.SetFederationHook(hook)

	require.NoError(t, svc.AcceptRequest("alice", "bob"))
	assert.Equal(t, []string{"bob->alice"}, hook.accepts)
}

func TestService_FederationHook_OnAccept_UserLookupFailure(t *testing.T) {
	svc, ur, _, frRepo := newSvc(t)
	addUser(t, ur, "alice", true)
	addUser(t, ur, "bob", false)
	frRepo.Requests["req"] = &model.FollowRequest{ID: "req", FollowerID: "bob", FolloweeID: "alice"}
	hook := &recordingFederationHook{}
	svc.SetFederationHook(hook)

	// follower (bob) を消してから accept する → hook 呼ばれない
	delete(ur.Users, "bob")
	require.NoError(t, svc.AcceptRequest("alice", "bob"))
	assert.Empty(t, hook.accepts)
}
