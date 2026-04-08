package chart

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// randReader is overridden in tests to exercise the rand.Read fallback
// path inside newLockToken.
var randReader io.Reader = rand.Reader

// Locker acquires short-lived mutual exclusion for chart bucket inserts.
// Without it two server instances racing to insert the "current hour"
// row for the same chart can each create a row, and the unique index
// blow-up is recovered only by an error retry. The Locker is used in
// claimCurrentLog around the read-then-insert section.
type Locker interface {
	// Acquire blocks until the named lock is held or ctx is cancelled.
	// The returned release function MUST be called exactly once to
	// release the lock. The TTL acts as a safety net in case the caller
	// crashes before releasing.
	Acquire(ctx context.Context, key string, ttl time.Duration) (release func(), err error)
}

// RedisLocker implements Locker on top of a redis.Client using SETNX +
// expiry. The implementation is intentionally simple — no Redlock
// quorum, no token-based fencing — because chart inserts are
// idempotent at the row level (the unique index protects against
// double inserts even if the lock is wrong).
type RedisLocker struct {
	client *redis.Client
	prefix string
}

// NewRedisLocker constructs a RedisLocker. `prefix` is prepended to all
// lock keys so multiple chart instances or test runs can share a Redis
// without colliding.
func NewRedisLocker(client *redis.Client, prefix string) *RedisLocker {
	if prefix == "" {
		prefix = "chartInsertLock:"
	}
	return &RedisLocker{client: client, prefix: prefix}
}

// Acquire spins on SETNX with exponential backoff up to ctx deadline.
// The token is a random 16-byte hex so the release script can verify
// ownership before deleting (preventing accidental release of a lock
// re-acquired by another caller after our TTL expired).
func (l *RedisLocker) Acquire(ctx context.Context, key string, ttl time.Duration) (func(), error) {
	if l == nil || l.client == nil {
		return nil, errors.New("chart: redis locker is not configured")
	}
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	token := newLockToken()
	full := l.prefix + key
	backoff := 5 * time.Millisecond
	for {
		// SET NX ... EX ... を使う (SetNX は古い API 経由なので Set + SetArgs)
		res, err := l.client.SetArgs(ctx, full, token, redis.SetArgs{
			Mode: "NX",
			TTL:  ttl,
		}).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return nil, err
		}
		if res == "OK" {
			release := func() {
				// 安全に解放するため、自分のtokenが残っているときだけDELする。
				// 他者がTTL切れ後に再取得していた場合は何もしない。
				releaseScript.Run(context.Background(), l.client, []string{full}, token)
			}
			return release, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < 200*time.Millisecond {
			backoff *= 2
		}
	}
}

// releaseScript atomically deletes the lock key only if its current
// value matches our token. This prevents one caller from accidentally
// releasing another caller's lock after a TTL expiry.
var releaseScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("del", KEYS[1])
else
    return 0
end
`)

func newLockToken() string {
	var b [16]byte
	// rand.Reader は通常失敗しないが、エントロピー枯渇に備えて時刻ベースの
	// フォールバックを用意する。テストでは randReader を io.ReadFull が
	// 失敗する Reader に差し替えてこの分岐を踏む。
	if _, err := io.ReadFull(randReader, b[:]); err != nil {
		return time.Now().UTC().Format("20060102150405.000000")
	}
	return hex.EncodeToString(b[:])
}

// MemoryLocker is a process-local Locker for use in tests and for the
// Noop chart engine. It uses sync.Mutex per key.
type MemoryLocker struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

// NewMemoryLocker constructs a MemoryLocker.
func NewMemoryLocker() *MemoryLocker {
	return &MemoryLocker{locks: make(map[string]*sync.Mutex)}
}

// Acquire blocks until the per-key mutex is held. The TTL is ignored
// because in-process locks cannot deadlock against a crashed peer.
func (l *MemoryLocker) Acquire(_ context.Context, key string, _ time.Duration) (func(), error) {
	l.mu.Lock()
	m, ok := l.locks[key]
	if !ok {
		m = &sync.Mutex{}
		l.locks[key] = m
	}
	l.mu.Unlock()
	m.Lock()
	return func() { m.Unlock() }, nil
}
