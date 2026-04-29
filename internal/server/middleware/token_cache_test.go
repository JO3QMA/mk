package middleware

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
)

// fakeClock returns a stub `now` function whose value can be advanced from
// the test. Used to drive TTL expiry without sleeping.
type fakeClock struct {
	t atomic.Int64 // unix nanoseconds
}

func newFakeClock(start time.Time) *fakeClock {
	c := &fakeClock{}
	c.t.Store(start.UnixNano())
	return c
}

func (c *fakeClock) Now() time.Time { return time.Unix(0, c.t.Load()) }
func (c *fakeClock) Advance(d time.Duration) {
	c.t.Add(int64(d))
}

func TestTokenCache_GetMiss(t *testing.T) {
	c := newTokenCache()
	_, ok := c.get("ghost")
	assert.False(t, ok)
}

func TestTokenCache_PutGet(t *testing.T) {
	c := newTokenCache()
	u := &model.User{ID: "u1"}
	c.put("tok", u)
	got, ok := c.get("tok")
	assert.True(t, ok)
	assert.Same(t, u, got)
}

func TestTokenCache_TTLExpired(t *testing.T) {
	clock := newFakeClock(time.Unix(1_700_000_000, 0))
	c := newTokenCache()
	c.now = clock.Now
	c.put("tok", &model.User{ID: "u1"})

	clock.Advance(authCacheTTL + time.Second)
	_, ok := c.get("tok")
	assert.False(t, ok, "expired entry must be evicted on get")
	assert.Equal(t, 0, c.len(), "expired entry should be deleted from map")
}

func TestTokenCache_Sweep(t *testing.T) {
	clock := newFakeClock(time.Unix(1_700_000_000, 0))
	c := newTokenCache()
	c.now = clock.Now
	c.sweepEvery = 0 // disable auto-sweep so we can call it explicitly
	c.put("a", &model.User{ID: "ua"})
	c.put("b", &model.User{ID: "ub"})
	clock.Advance(authCacheTTL + time.Second)
	c.put("c", &model.User{ID: "uc"}) // c is fresh
	c.sweep()

	assert.Equal(t, 1, c.len(), "only c should survive sweep")
	_, ok := c.get("c")
	assert.True(t, ok)
}

func TestTokenCache_AutoSweepOnPut(t *testing.T) {
	clock := newFakeClock(time.Unix(1_700_000_000, 0))
	c := newTokenCache()
	c.now = clock.Now
	c.sweepEvery = 4
	for i := 0; i < 3; i++ {
		c.put("stale-"+string(rune('a'+i)), &model.User{ID: "u"})
	}
	clock.Advance(authCacheTTL + time.Second)
	// 4th put should hit the sweep threshold and clean the 3 stale entries.
	c.put("fresh", &model.User{ID: "fresh"})
	assert.Equal(t, 1, c.len(), "only the fresh entry should remain after auto-sweep")
}

func TestTokenCache_Invalidate(t *testing.T) {
	c := newTokenCache()
	c.put("tok", &model.User{ID: "u1"})
	c.invalidate("tok")
	_, ok := c.get("tok")
	assert.False(t, ok)
}
