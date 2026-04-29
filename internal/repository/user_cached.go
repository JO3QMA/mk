package repository

import (
	"errors"
	"sync"
	"time"

	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// CachedUserRepository wraps a UserRepository with a time-based per-userID
// cache for FindByID + FindProfileByUserID. ShowByID 経路はタイムライン描画
// で大量に呼ばれるが、user / user_profile 行の更新頻度は低いので、5 分 TTL
// + 全 mutation で per-userID invalidate するだけで hit 率がほぼ 100% に
// 張り付く (#300 3-3)。
//
// In-memory wrapper を選択した理由:
//   - role / emoji / meta cache (#300 3-5 / 3-6 / 3-1) と同じ既存パターンに
//     揃えるため。Redis 越しの user cache はインスタンス間共有という別の
//     利点があるが、out-of-band write が無い前提では in-memory で十分。
//   - 全 user 系 mutation が `userRepo` 経由 (admin, federation processor,
//     following service, signin, resetpassword 等) なので wrapper 層で
//     invalidate を保証できる。DB 直接 UPDATE の経路は無い。
//
// 注意: cached value はポインタ参照を直接返す。caller が User / UserProfile
// を mutate すると他の caller の view を破壊するので **read-only として扱う**。
// 既存実装も `FindByID` / `FindProfileByUserID` の戻り値を mutate していない
// (mutation は UpdateUser / UpdateProfile を介する) ので不変条件は保たれる。
type CachedUserRepository struct {
	UserRepository
	ttl time.Duration

	mu       sync.RWMutex
	users    map[string]userCacheEntry
	profiles map[string]profileCacheEntry
}

type userCacheEntry struct {
	user      *model.User
	expiresAt time.Time
	// missing=true は "DB に存在しない" を表す negative cache。pos は
	// nil pointer と区別したいので別フラグで持つ。
	missing bool
}

type profileCacheEntry struct {
	profile   *model.UserProfile
	expiresAt time.Time
	missing   bool
}

// userCacheTTL は CachedUserRepository のデフォルト TTL。短すぎると
// timeline 描画で hit 率が落ち、長すぎると out-of-band の (例: 別の mk
// プロセスが共有 DB に書き込む) write が反映されるまで lag が出る。
// admin / federation processor / signin はすべて当該 wrapper 経由なので、
// 単一プロセス運用では TTL で守る必要は無いが、保守的に 5 分を採用する。
const userCacheTTL = 5 * time.Minute

// NewCachedUserRepository wraps inner with the default TTL.
func NewCachedUserRepository(inner UserRepository) *CachedUserRepository {
	return NewCachedUserRepositoryWithTTL(inner, userCacheTTL)
}

// NewCachedUserRepositoryWithTTL is the test-friendly constructor.
func NewCachedUserRepositoryWithTTL(inner UserRepository, ttl time.Duration) *CachedUserRepository {
	return &CachedUserRepository{
		UserRepository: inner,
		ttl:            ttl,
		users:          make(map[string]userCacheEntry),
		profiles:       make(map[string]profileCacheEntry),
	}
}

// invalidate drops both user and profile entries for the given userID.
// Mutation 系メソッドの末尾で呼ぶ。
func (c *CachedUserRepository) invalidate(userID string) {
	if userID == "" {
		return
	}
	c.mu.Lock()
	delete(c.users, userID)
	delete(c.profiles, userID)
	c.mu.Unlock()
}

// Invalidate is the public counterpart of invalidate. Out-of-band code paths
// that bypass this wrapper (例: テスト) can call it to force a refresh.
func (c *CachedUserRepository) Invalidate(userID string) {
	c.invalidate(userID)
}

// FindByID returns the cached user for id, falling through to the inner repo
// on miss / expiry. Errors other than not-found are returned unchanged and
// **never cached** so transient DB errors recover on the next call.
func (c *CachedUserRepository) FindByID(id string) (*model.User, error) {
	if id == "" {
		return c.UserRepository.FindByID(id)
	}
	c.mu.RLock()
	if e, ok := c.users[id]; ok && time.Now().Before(e.expiresAt) {
		c.mu.RUnlock()
		if e.missing {
			return nil, gorm.ErrRecordNotFound
		}
		return e.user, nil
	}
	c.mu.RUnlock()

	u, err := c.UserRepository.FindByID(id)
	if err != nil {
		// 「見つからない」は negative-cache する。timeline 描画で
		// 削除済みユーザーへの参照が大量に来る可能性に備える。それ
		// 以外の err は cache せず caller に返して上位で扱わせる。
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.storeUserMissing(id)
			return nil, err
		}
		return nil, err
	}
	c.storeUser(id, u)
	return u, nil
}

// FindProfileByUserID returns the cached profile for userID with the same
// negative-cache semantics as FindByID.
func (c *CachedUserRepository) FindProfileByUserID(userID string) (*model.UserProfile, error) {
	if userID == "" {
		return c.UserRepository.FindProfileByUserID(userID)
	}
	c.mu.RLock()
	if e, ok := c.profiles[userID]; ok && time.Now().Before(e.expiresAt) {
		c.mu.RUnlock()
		if e.missing {
			return nil, gorm.ErrRecordNotFound
		}
		return e.profile, nil
	}
	c.mu.RUnlock()

	p, err := c.UserRepository.FindProfileByUserID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.storeProfileMissing(userID)
			return nil, err
		}
		return nil, err
	}
	c.storeProfile(userID, p)
	return p, nil
}

func (c *CachedUserRepository) storeUser(id string, u *model.User) {
	c.mu.Lock()
	c.users[id] = userCacheEntry{user: u, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

func (c *CachedUserRepository) storeUserMissing(id string) {
	c.mu.Lock()
	c.users[id] = userCacheEntry{missing: true, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

func (c *CachedUserRepository) storeProfile(userID string, p *model.UserProfile) {
	c.mu.Lock()
	c.profiles[userID] = profileCacheEntry{profile: p, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

func (c *CachedUserRepository) storeProfileMissing(userID string) {
	c.mu.Lock()
	c.profiles[userID] = profileCacheEntry{missing: true, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

// --- mutating methods: delegate then invalidate -----------------------------

func (c *CachedUserRepository) UpdateUser(userID string, fields map[string]any) error {
	if err := c.UserRepository.UpdateUser(userID, fields); err != nil {
		return err
	}
	c.invalidate(userID)
	return nil
}

func (c *CachedUserRepository) UpdateProfile(userID string, fields map[string]any) error {
	if err := c.UserRepository.UpdateProfile(userID, fields); err != nil {
		return err
	}
	c.invalidate(userID)
	return nil
}

func (c *CachedUserRepository) CreateProfile(profile *model.UserProfile) error {
	if err := c.UserRepository.CreateProfile(profile); err != nil {
		return err
	}
	if profile != nil {
		c.invalidate(profile.UserID)
	}
	return nil
}

func (c *CachedUserRepository) IncrementFollowingCount(userID string, delta int) error {
	if err := c.UserRepository.IncrementFollowingCount(userID, delta); err != nil {
		return err
	}
	c.invalidate(userID)
	return nil
}

func (c *CachedUserRepository) IncrementFollowersCount(userID string, delta int) error {
	if err := c.UserRepository.IncrementFollowersCount(userID, delta); err != nil {
		return err
	}
	c.invalidate(userID)
	return nil
}
