package mkqdriver

import (
	"math"
	"math/rand/v2"
	"time"
)

// httpRelatedBackoffBaseDelay / httpRelatedBackoffCap / httpRelatedBackoffJitter
// mirror Misskey TS QueueProcessorService.httpRelatedBackoff verbatim:
//
//	const baseDelay = 60 * 1000;            // 1min
//	const maxBackoff = 8 * 60 * 60 * 1000;  // 8hours
//	let backoff = (2 ** attemptsMade - 1) * baseDelay;
//	backoff = Math.min(backoff, maxBackoff);
//	backoff += Math.round(backoff * Math.random() * 0.2);
//
// deliver / inbox の retry を TS と drop-in 一致させるため (#1406, mkq#67)。
const (
	httpRelatedBackoffBaseDelay = time.Minute
	httpRelatedBackoffCap       = 8 * time.Hour
	httpRelatedBackoffJitter    = 0.2
)

// httpRelatedBackoff reproduces Misskey TS's httpRelatedBackoff. attemptsMade
// is the post-bump attempt count (BullMQ settings.backoffStrategy semantics).
// rng はテストで決定的に差し替えられるよう引数で受け取る (nil = math/rand/v2)。
func httpRelatedBackoff(attemptsMade int, rng func() float64) time.Duration {
	if rng == nil {
		rng = rand.Float64
	}
	// (2^attemptsMade - 1) * baseDelay。attemptsMade が大きいと 2^n が
	// float64 でも巨大になるが、直後に cap で 8h に丸めるので overflow は
	// 問題にならない (cap 比較は time.Duration の範囲内)。
	mult := math.Pow(2, float64(attemptsMade)) - 1
	backoff := time.Duration(mult * float64(httpRelatedBackoffBaseDelay))
	if backoff > httpRelatedBackoffCap || backoff < 0 {
		backoff = httpRelatedBackoffCap
	}
	// +0..20% jitter。TS は Math.round するが、ns 単位の丸め差は無視できる
	// ため float のまま加算する。
	backoff += time.Duration(float64(backoff) * rng() * httpRelatedBackoffJitter)
	return backoff
}
