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
	// maxEntries は users / profiles map それぞれの soft cap。size がこれを
	// 超えたタイミングの insert で expired entry を一括削除する。0 なら
	// userCacheMaxEntries が使われる。
	maxEntries int

	mu       sync.RWMutex
	users    map[string]userCacheEntry
	profiles map[string]profileCacheEntry
	// byURI は AP 由来 URI → user のキャッシュ。inbox worker の hot path
	// (federation.Resolver.ResolveActor) が毎リクエスト FindByURI を叩く
	// ため、これを cache せずに DB に行くと PR #565 で軽量化した HTTP
	// 受信 fast path に対して worker drain がボトルネックになる (#xxx)。
	byURI map[string]uriCacheEntry
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

type uriCacheEntry struct {
	user      *model.User
	expiresAt time.Time
	missing   bool
}

// userCacheTTL は CachedUserRepository のデフォルト TTL。短すぎると
// timeline 描画で hit 率が落ち、長すぎると out-of-band の (例: 別の mk
// プロセスが共有 DB に書き込む) write が反映されるまで lag が出る。
// admin / federation processor / signin はすべて当該 wrapper 経由なので、
// 単一プロセス運用では TTL で守る必要は無いが、保守的に 5 分を採用する。
const userCacheTTL = 5 * time.Minute

// userCacheMaxEntries は per-map (users / profiles) のエントリ上限。
// 連合インスタンスでは reachable な remote user 数だけ unique ID lookup が
// 走り得るので、エントリは monotonically に増え続ける。しきい値超過時に
// 期限切れエントリの一括 sweep を走らせて O(1) amortized で抑える。
// しきい値 50000 は per-entry ~256B 想定で ~12MB を上限に取る (Devin
// review #552: unbounded growth 対策)。
const userCacheMaxEntries = 50000

// NewCachedUserRepository wraps inner with the default TTL.
func NewCachedUserRepository(inner UserRepository) *CachedUserRepository {
	return NewCachedUserRepositoryWithTTL(inner, userCacheTTL)
}

// NewCachedUserRepositoryWithTTL is the test-friendly constructor.
func NewCachedUserRepositoryWithTTL(inner UserRepository, ttl time.Duration) *CachedUserRepository {
	return newCachedUserRepository(inner, ttl, userCacheMaxEntries)
}

// newCachedUserRepository is the internal constructor that exposes maxEntries.
// Test-only entry point so the eviction path can be exercised without
// inserting 50k synthetic rows.
func newCachedUserRepository(inner UserRepository, ttl time.Duration, maxEntries int) *CachedUserRepository {
	if maxEntries <= 0 {
		maxEntries = userCacheMaxEntries
	}
	return &CachedUserRepository{
		UserRepository: inner,
		ttl:            ttl,
		maxEntries:     maxEntries,
		users:          make(map[string]userCacheEntry),
		profiles:       make(map[string]profileCacheEntry),
		byURI:          make(map[string]uriCacheEntry),
	}
}

// SetMaxEntriesForTest overrides the soft cap. Tests only.
func (c *CachedUserRepository) SetMaxEntriesForTest(n int) {
	c.mu.Lock()
	if n <= 0 {
		c.maxEntries = userCacheMaxEntries
	} else {
		c.maxEntries = n
	}
	c.mu.Unlock()
}

// invalidate drops both user and profile entries for the given userID.
// Mutation 系メソッドの末尾で呼ぶ。byURI は per-userID で逆引きせず TTL に
// 任せる (URI は actor の canonical id なので user lifetime 中は不変、
// 古い entry が短時間残っても害は無い)。
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
	c.evictExpiredUsersLocked()
	c.users[id] = userCacheEntry{user: u, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

func (c *CachedUserRepository) storeUserMissing(id string) {
	c.mu.Lock()
	c.evictExpiredUsersLocked()
	c.users[id] = userCacheEntry{missing: true, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

func (c *CachedUserRepository) storeProfile(userID string, p *model.UserProfile) {
	c.mu.Lock()
	c.evictExpiredProfilesLocked()
	c.profiles[userID] = profileCacheEntry{profile: p, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

func (c *CachedUserRepository) storeProfileMissing(userID string) {
	c.mu.Lock()
	c.evictExpiredProfilesLocked()
	c.profiles[userID] = profileCacheEntry{missing: true, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

// FindByURI returns the cached user for a remote AP URI, falling through
// to the inner repo on miss / expiry. AP inbox worker の hot path から
// 毎 request で呼ばれるため cache が effective: 同じ remote actor 由来の
// activity が連続して届く federation 受信は典型的に高 hit-ratio。
//
// negative cache 含む semantic は FindByID と同じ。URI は actor の
// canonical id (lifetime-stable) なので invalidate は通常 TTL 任せだが、
// 別経路で user row が更新された場合に refresh されるよう同期 store する
// FindByID 経路と TTL を揃える。
func (c *CachedUserRepository) FindByURI(uri string) (*model.User, error) {
	if uri == "" {
		return c.UserRepository.FindByURI(uri)
	}
	c.mu.RLock()
	if e, ok := c.byURI[uri]; ok && time.Now().Before(e.expiresAt) {
		c.mu.RUnlock()
		if e.missing {
			return nil, gorm.ErrRecordNotFound
		}
		return e.user, nil
	}
	c.mu.RUnlock()

	u, err := c.UserRepository.FindByURI(uri)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.storeURIMissing(uri)
			return nil, err
		}
		return nil, err
	}
	c.storeURI(uri, u)
	// FindByID 経路の cache にも入れておく。worker は両方の lookup を
	// する経路があり、片方 miss → DB → 反対側だけ updated は無駄なので。
	c.storeUser(u.ID, u)
	return u, nil
}

func (c *CachedUserRepository) storeURI(uri string, u *model.User) {
	c.mu.Lock()
	c.evictExpiredURIsLocked()
	c.byURI[uri] = uriCacheEntry{user: u, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

func (c *CachedUserRepository) storeURIMissing(uri string) {
	c.mu.Lock()
	c.evictExpiredURIsLocked()
	c.byURI[uri] = uriCacheEntry{missing: true, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

// evictExpiredUsersLocked は users map のサイズが userCacheMaxEntries を
// 超えていたら、期限切れエントリを一括削除して memory growth を抑える。
// 呼び出し側で c.mu.Lock 済みである必要がある。Devin #552: unbounded
// growth 対策。
func (c *CachedUserRepository) evictExpiredUsersLocked() {
	if len(c.users) < c.maxEntries {
		return
	}
	now := time.Now()
	for k, e := range c.users {
		if now.After(e.expiresAt) {
			delete(c.users, k)
		}
	}
}

func (c *CachedUserRepository) evictExpiredProfilesLocked() {
	if len(c.profiles) < c.maxEntries {
		return
	}
	now := time.Now()
	for k, e := range c.profiles {
		if now.After(e.expiresAt) {
			delete(c.profiles, k)
		}
	}
}

func (c *CachedUserRepository) evictExpiredURIsLocked() {
	if len(c.byURI) < c.maxEntries {
		return
	}
	now := time.Now()
	for k, e := range c.byURI {
		if now.After(e.expiresAt) {
			delete(c.byURI, k)
		}
	}
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

func (c *CachedUserRepository) IncrementNotesCount(userID string, delta int) error {
	if err := c.UserRepository.IncrementNotesCount(userID, delta); err != nil {
		return err
	}
	c.invalidate(userID)
	return nil
}

// FindManyByIDs delegates to the inner repo and warms the per-userID cache
// with the returned rows. これにより users/show バルク経路 (#503) の結果が
// 直後の ShowByID / FindByID で hit するようになる (Devin review #552 FLAG-1)。
// missing rows は inner が silently skip するため negative-cache は付けない
// (どの ID が抜けたかは戻り値からは判別できないため、別 round-trip での
// FindByID が走った時に negative-cache する経路に任せる)。
func (c *CachedUserRepository) FindManyByIDs(ids []string) ([]*model.User, error) {
	users, err := c.UserRepository.FindManyByIDs(ids)
	if err != nil {
		return nil, err
	}
	if len(users) > 0 {
		c.mu.Lock()
		c.evictExpiredUsersLocked()
		expiry := time.Now().Add(c.ttl)
		for _, u := range users {
			if u != nil && u.ID != "" {
				c.users[u.ID] = userCacheEntry{user: u, expiresAt: expiry}
			}
		}
		c.mu.Unlock()
	}
	return users, nil
}

// FindProfilesByUserIDs is the profile counterpart of FindManyByIDs:
// バルクで返ってきた行を per-userID cache に warm する。
func (c *CachedUserRepository) FindProfilesByUserIDs(userIDs []string) ([]*model.UserProfile, error) {
	profiles, err := c.UserRepository.FindProfilesByUserIDs(userIDs)
	if err != nil {
		return nil, err
	}
	if len(profiles) > 0 {
		c.mu.Lock()
		c.evictExpiredProfilesLocked()
		expiry := time.Now().Add(c.ttl)
		for _, p := range profiles {
			if p != nil && p.UserID != "" {
				c.profiles[p.UserID] = profileCacheEntry{profile: p, expiresAt: expiry}
			}
		}
		c.mu.Unlock()
	}
	return profiles, nil
}
