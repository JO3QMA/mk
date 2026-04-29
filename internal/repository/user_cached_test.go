package repository_test

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// countingUserRepo wraps a stub UserRepository and exposes call counts on
// the methods we cache or invalidate around. それ以外は zero-value pass-
// through で十分 (caching とは無関係なので)。
type countingUserRepo struct {
	repository.UserRepository

	users    map[string]*model.User
	profiles map[string]*model.UserProfile

	findByIDCalls            atomic.Int64
	findProfileByUserIDCalls atomic.Int64

	updateUserErr      error
	updateProfileErr   error
	createProfileErr   error
	incFollowingErr    error
	incFollowersErr    error
	updateUserCalls    atomic.Int64
	updateProfileCalls atomic.Int64
}

func newCountingUserRepo() *countingUserRepo {
	return &countingUserRepo{
		users:    make(map[string]*model.User),
		profiles: make(map[string]*model.UserProfile),
	}
}

func (c *countingUserRepo) FindByID(id string) (*model.User, error) {
	c.findByIDCalls.Add(1)
	if u, ok := c.users[id]; ok {
		return u, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (c *countingUserRepo) FindProfileByUserID(userID string) (*model.UserProfile, error) {
	c.findProfileByUserIDCalls.Add(1)
	if p, ok := c.profiles[userID]; ok {
		return p, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (c *countingUserRepo) UpdateUser(userID string, _ map[string]any) error {
	c.updateUserCalls.Add(1)
	return c.updateUserErr
}

func (c *countingUserRepo) UpdateProfile(userID string, _ map[string]any) error {
	c.updateProfileCalls.Add(1)
	return c.updateProfileErr
}

func (c *countingUserRepo) CreateProfile(p *model.UserProfile) error {
	if c.createProfileErr != nil {
		return c.createProfileErr
	}
	if p != nil {
		c.profiles[p.UserID] = p
	}
	return nil
}

func (c *countingUserRepo) IncrementFollowingCount(_ string, _ int) error {
	return c.incFollowingErr
}

func (c *countingUserRepo) IncrementFollowersCount(_ string, _ int) error {
	return c.incFollowersErr
}

func TestCachedUserRepository_FindByIDHitsInnerOnce(t *testing.T) {
	inner := newCountingUserRepo()
	inner.users["u1"] = &model.User{ID: "u1", Username: "alice"}
	cached := repository.NewCachedUserRepository(inner)

	for i := 0; i < 10; i++ {
		got, err := cached.FindByID("u1")
		require.NoError(t, err)
		assert.Equal(t, "alice", got.Username)
	}
	assert.Equal(t, int64(1), inner.findByIDCalls.Load(),
		"10 cached lookups must hit inner exactly once")
}

func TestCachedUserRepository_FindProfileByUserIDHitsInnerOnce(t *testing.T) {
	inner := newCountingUserRepo()
	desc := "hi"
	inner.profiles["u1"] = &model.UserProfile{UserID: "u1", Description: &desc}
	cached := repository.NewCachedUserRepository(inner)

	for i := 0; i < 5; i++ {
		got, err := cached.FindProfileByUserID("u1")
		require.NoError(t, err)
		assert.Equal(t, "hi", *got.Description)
	}
	assert.Equal(t, int64(1), inner.findProfileByUserIDCalls.Load())
}

func TestCachedUserRepository_NotFoundIsNegativeCached(t *testing.T) {
	inner := newCountingUserRepo()
	cached := repository.NewCachedUserRepository(inner)

	for i := 0; i < 5; i++ {
		_, err := cached.FindByID("ghost")
		require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	}
	assert.Equal(t, int64(1), inner.findByIDCalls.Load(),
		"missing rows must be negative-cached so timeline ghost lookups don't reflood inner")
}

// ErrNotFound 以外の error は cache されない。transient な DB 障害が次回
// 呼び出しで自動回復することを担保する。
type erroringUserRepo struct {
	repository.UserRepository
	calls atomic.Int64
}

func (e *erroringUserRepo) FindByID(_ string) (*model.User, error) {
	e.calls.Add(1)
	return nil, errors.New("db down")
}

func (e *erroringUserRepo) FindProfileByUserID(_ string) (*model.UserProfile, error) {
	e.calls.Add(1)
	return nil, errors.New("db down")
}

func TestCachedUserRepository_TransientErrorIsNotCached(t *testing.T) {
	inner := &erroringUserRepo{}
	cached := repository.NewCachedUserRepository(inner)

	_, err := cached.FindByID("u1")
	require.Error(t, err)
	_, err = cached.FindByID("u1")
	require.Error(t, err)
	assert.Equal(t, int64(2), inner.calls.Load(),
		"non-NotFound errors must not be cached")

	_, err = cached.FindProfileByUserID("u1")
	require.Error(t, err)
	_, err = cached.FindProfileByUserID("u1")
	require.Error(t, err)
	assert.Equal(t, int64(4), inner.calls.Load())
}

func TestCachedUserRepository_RefreshesAfterTTL(t *testing.T) {
	inner := newCountingUserRepo()
	inner.users["u1"] = &model.User{ID: "u1"}
	cached := repository.NewCachedUserRepositoryWithTTL(inner, 1*time.Millisecond)

	_, err := cached.FindByID("u1")
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)
	_, err = cached.FindByID("u1")
	require.NoError(t, err)
	assert.Equal(t, int64(2), inner.findByIDCalls.Load())
}

func TestCachedUserRepository_UpdateUserInvalidates(t *testing.T) {
	inner := newCountingUserRepo()
	inner.users["u1"] = &model.User{ID: "u1", Username: "alice"}
	cached := repository.NewCachedUserRepository(inner)

	_, _ = cached.FindByID("u1") // warm
	require.NoError(t, cached.UpdateUser("u1", map[string]any{"name": "x"}))
	_, _ = cached.FindByID("u1")
	assert.Equal(t, int64(2), inner.findByIDCalls.Load(),
		"UpdateUser must invalidate so the next FindByID re-reads")
}

func TestCachedUserRepository_UpdateUserErrDoesNotInvalidate(t *testing.T) {
	inner := newCountingUserRepo()
	inner.users["u1"] = &model.User{ID: "u1"}
	inner.updateUserErr = errors.New("db down")
	cached := repository.NewCachedUserRepository(inner)

	_, _ = cached.FindByID("u1") // warm
	err := cached.UpdateUser("u1", map[string]any{"name": "x"})
	require.Error(t, err)
	_, _ = cached.FindByID("u1")
	assert.Equal(t, int64(1), inner.findByIDCalls.Load(),
		"failed UpdateUser must not invalidate (DB state unchanged)")
}

func TestCachedUserRepository_UpdateProfileInvalidatesProfile(t *testing.T) {
	inner := newCountingUserRepo()
	desc := "hi"
	inner.profiles["u1"] = &model.UserProfile{UserID: "u1", Description: &desc}
	cached := repository.NewCachedUserRepository(inner)

	_, _ = cached.FindProfileByUserID("u1")
	require.NoError(t, cached.UpdateProfile("u1", map[string]any{"desc": "y"}))
	_, _ = cached.FindProfileByUserID("u1")
	assert.Equal(t, int64(2), inner.findProfileByUserIDCalls.Load())
}

// UpdateProfile (例: lastActiveDate / password) が user 行も同 ID で
// 共有されているケースに備え、profile 更新時は user 側 cache も飛ばす。
// (現状は同 userID で削除しているので user 側エントリも消える挙動。)
func TestCachedUserRepository_UpdateProfileInvalidatesUserAlso(t *testing.T) {
	inner := newCountingUserRepo()
	inner.users["u1"] = &model.User{ID: "u1"}
	desc := "hi"
	inner.profiles["u1"] = &model.UserProfile{UserID: "u1", Description: &desc}
	cached := repository.NewCachedUserRepository(inner)

	_, _ = cached.FindByID("u1")
	_, _ = cached.FindProfileByUserID("u1")
	require.NoError(t, cached.UpdateProfile("u1", map[string]any{"desc": "y"}))
	_, _ = cached.FindByID("u1")
	assert.Equal(t, int64(2), inner.findByIDCalls.Load(),
		"UpdateProfile invalidates both user and profile entries for the same ID")
}

func TestCachedUserRepository_CreateProfileInvalidates(t *testing.T) {
	inner := newCountingUserRepo()
	cached := repository.NewCachedUserRepository(inner)

	// 最初は profile なし → negative cache に乗る
	_, err := cached.FindProfileByUserID("u1")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	require.Equal(t, int64(1), inner.findProfileByUserIDCalls.Load())

	// CreateProfile 後は negative cache を飛ばして再取得する
	require.NoError(t, cached.CreateProfile(&model.UserProfile{UserID: "u1"}))
	got, err := cached.FindProfileByUserID("u1")
	require.NoError(t, err)
	assert.Equal(t, "u1", got.UserID)
	assert.Equal(t, int64(2), inner.findProfileByUserIDCalls.Load())
}

func TestCachedUserRepository_IncrementFollowingCountInvalidates(t *testing.T) {
	inner := newCountingUserRepo()
	inner.users["u1"] = &model.User{ID: "u1", FollowingCount: 0}
	cached := repository.NewCachedUserRepository(inner)

	_, _ = cached.FindByID("u1")
	require.NoError(t, cached.IncrementFollowingCount("u1", 1))
	_, _ = cached.FindByID("u1")
	assert.Equal(t, int64(2), inner.findByIDCalls.Load())
}

func TestCachedUserRepository_IncrementFollowersCountInvalidates(t *testing.T) {
	inner := newCountingUserRepo()
	inner.users["u1"] = &model.User{ID: "u1"}
	cached := repository.NewCachedUserRepository(inner)

	_, _ = cached.FindByID("u1")
	require.NoError(t, cached.IncrementFollowersCount("u1", 1))
	_, _ = cached.FindByID("u1")
	assert.Equal(t, int64(2), inner.findByIDCalls.Load())
}

func TestCachedUserRepository_PublicInvalidate(t *testing.T) {
	inner := newCountingUserRepo()
	inner.users["u1"] = &model.User{ID: "u1"}
	cached := repository.NewCachedUserRepositoryWithTTL(inner, time.Hour)

	_, _ = cached.FindByID("u1")
	cached.Invalidate("u1")
	_, _ = cached.FindByID("u1")
	assert.Equal(t, int64(2), inner.findByIDCalls.Load())
}

// 空 ID は no-op で inner にだけ任せる (lookup error がそのまま返る)。
// 空 ID を cache key にすると意図しない衝突を起こすので意図的に bypass する。
func TestCachedUserRepository_EmptyIDIsBypassed(t *testing.T) {
	inner := newCountingUserRepo()
	cached := repository.NewCachedUserRepository(inner)

	for i := 0; i < 3; i++ {
		_, _ = cached.FindByID("")
		_, _ = cached.FindProfileByUserID("")
	}
	assert.Equal(t, int64(3), inner.findByIDCalls.Load())
	assert.Equal(t, int64(3), inner.findProfileByUserIDCalls.Load())
}
