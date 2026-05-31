package mkqdriver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// httpRelatedBackoff が Misskey TS の式 (2^n-1)*60s を jitter=0 で再現する
// ことを確認する。rng を 0 固定にして jitter 項を消す (#1406, mkq#67)。
func TestHTTPRelatedBackoff_Formula(t *testing.T) {
	zero := func() float64 { return 0 }
	cases := []struct {
		attemptsMade int
		want         time.Duration
	}{
		{1, time.Minute},      // (2^1-1)*60s = 60s
		{2, 3 * time.Minute},  // (2^2-1)*60s = 180s
		{3, 7 * time.Minute},  // (2^3-1)*60s = 420s
		{4, 15 * time.Minute}, // (2^4-1)*60s = 900s
	}
	for _, c := range cases {
		got := httpRelatedBackoff(c.attemptsMade, zero)
		assert.Equal(t, c.want, got, "attemptsMade=%d", c.attemptsMade)
	}
}

// 大きい attemptsMade では 8h cap に丸められる (jitter=0)。
func TestHTTPRelatedBackoff_Cap(t *testing.T) {
	zero := func() float64 { return 0 }
	// 2^20 * 60s は 8h を遥かに超えるので cap される。
	got := httpRelatedBackoff(20, zero)
	assert.Equal(t, httpRelatedBackoffCap, got)
}

// jitter は base に対して最大 +20% 上乗せされる。rng=1 で上限を確認する。
func TestHTTPRelatedBackoff_JitterUpperBound(t *testing.T) {
	one := func() float64 { return 1 }
	// attemptsMade=1: base=60s, +20% = 72s
	got := httpRelatedBackoff(1, one)
	assert.Equal(t, 72*time.Second, got)
}

// jitter=0..1 の範囲で結果が [base, base*1.2] に収まる。
func TestHTTPRelatedBackoff_JitterRange(t *testing.T) {
	base := 3 * time.Minute // attemptsMade=2
	lo := httpRelatedBackoff(2, func() float64 { return 0 })
	hi := httpRelatedBackoff(2, func() float64 { return 1 })
	assert.Equal(t, base, lo)
	assert.Equal(t, base+time.Duration(float64(base)*0.2), hi)
	assert.True(t, hi > lo)
}
