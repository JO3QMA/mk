package middleware

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/shiroha-a/mk/internal/api/apierr"
)

// EndpointLimit defines rate limit parameters for an API endpoint.
type EndpointLimit struct {
	Duration    time.Duration // ウィンドウ幅（例: 1時間）
	Max         int           // ウィンドウ内最大リクエスト数
	MinInterval time.Duration // 連続リクエスト最小間隔（0なら無効）
}

// LimitInfo holds the result of a rate limit check.
type LimitInfo struct {
	Remaining int   // 残りリクエスト数
	ResetMs   int64 // リセット時刻 (Unix milliseconds)
}

// RateLimitStore abstracts the backing store for rate limit counters.
type RateLimitStore interface {
	// Check records a request and returns the current limit status.
	// key is the rate limit bucket identifier, duration is the window
	// size, and max is the maximum number of requests allowed.
	Check(ctx context.Context, key string, duration time.Duration, max int) (LimitInfo, error)
}

// RedisRateLimitStore implements RateLimitStore using Redis sorted sets.
// The algorithm mirrors the TS ratelimiter (visionmedia/node-ratelimiter):
// each request is a ZADD with a microsecond timestamp as both score and
// member, old entries are pruned with ZREMRANGEBYSCORE, and ZCARD gives
// the current count within the sliding window.
type RedisRateLimitStore struct {
	rdb *redis.Client
}

// NewRedisRateLimitStore creates a store backed by the given Redis client.
func NewRedisRateLimitStore(rdb *redis.Client) *RedisRateLimitStore {
	return &RedisRateLimitStore{rdb: rdb}
}

// Check implements RateLimitStore using a sliding-window sorted set.
func (s *RedisRateLimitStore) Check(ctx context.Context, key string, duration time.Duration, max int) (LimitInfo, error) {
	now := time.Now().UnixMicro()
	windowStart := now - duration.Microseconds()
	rkey := "limit:" + key

	pipe := s.rdb.Pipeline()
	pipe.ZRemRangeByScore(ctx, rkey, "0", fmt.Sprintf("%d", windowStart))
	zcardCmd := pipe.ZCard(ctx, rkey)
	pipe.ZAdd(ctx, rkey, redis.Z{Score: float64(now), Member: now})
	zrangeOldestCmd := pipe.ZRange(ctx, rkey, 0, 0)
	zrangeAtMaxCmd := pipe.ZRange(ctx, rkey, int64(-max), int64(-max))
	pipe.ZRemRangeByRank(ctx, rkey, 0, int64(-(max + 1)))
	pipe.PExpire(ctx, rkey, duration)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return LimitInfo{}, fmt.Errorf("rate limit redis pipeline: %w", err)
	}

	count := int(zcardCmd.Val())
	remaining := 0
	if count < max {
		remaining = max - count
	}

	// リセット時刻の計算: TS版と同じロジック
	// -max位置のエントリがあればそのタイムスタンプ、なければ最古のエントリ
	var resetMicro int64
	atMax := zrangeAtMaxCmd.Val()
	oldest := zrangeOldestCmd.Val()
	if len(atMax) > 0 {
		resetMicro = parseIntFromString(atMax[0]) + duration.Microseconds()
	} else if len(oldest) > 0 {
		resetMicro = parseIntFromString(oldest[0]) + duration.Microseconds()
	} else {
		resetMicro = now + duration.Microseconds()
	}

	return LimitInfo{
		Remaining: remaining,
		ResetMs:   resetMicro / 1000,
	}, nil
}

// parseIntFromString は文字列を int64 にパースする。
// Redis の ZRANGE は member を文字列で返すため。
func parseIntFromString(s string) int64 {
	var v int64
	fmt.Sscanf(s, "%d", &v)
	return v
}

// RateLimiter provides per-endpoint rate limiting as Echo middleware.
type RateLimiter struct {
	store             RateLimitStore
	enableIPRateLimit bool
	limits            map[string]*EndpointLimit
}

// NewRateLimiter creates a RateLimiter with the given store and config.
func NewRateLimiter(store RateLimitStore, enableIPRateLimit bool, limits map[string]*EndpointLimit) *RateLimiter {
	return &RateLimiter{
		store:             store,
		enableIPRateLimit: enableIPRateLimit,
		limits:            limits,
	}
}

// NewRedisRateLimiter creates a RateLimiter backed by Redis.
func NewRedisRateLimiter(rdb *redis.Client, enableIPRateLimit bool, limits map[string]*EndpointLimit) *RateLimiter {
	return NewRateLimiter(NewRedisRateLimitStore(rdb), enableIPRateLimit, limits)
}

// Middleware returns an Echo middleware that enforces rate limiting.
// リミット定義のないエンドポイントはそのまま通過する。
// Redisエラー時はfail-open（ログだけ出してリクエストを通す）。
func (rl *RateLimiter) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			endpoint := strings.TrimPrefix(c.Path(), "/api/")
			limit, ok := rl.limits[endpoint]
			if !ok {
				return next(c)
			}

			actor := rl.resolveActor(c)
			if actor == "" {
				return next(c)
			}

			ctx := c.Request().Context()

			// minIntervalチェック（設定されていれば）
			if limit.MinInterval > 0 {
				info, err := rl.store.Check(ctx, actor+":"+endpoint+":min", limit.MinInterval, 1)
				if err != nil {
					slog.Warn("rate limit store error (minInterval)", "endpoint", endpoint, "err", err)
					return next(c)
				}
				if info.Remaining == 0 {
					return rl.rejectRequest(c, info)
				}
			}

			// duration/maxチェック
			if limit.Duration > 0 && limit.Max > 0 {
				info, err := rl.store.Check(ctx, actor+":"+endpoint, limit.Duration, limit.Max)
				if err != nil {
					slog.Warn("rate limit store error", "endpoint", endpoint, "err", err)
					return next(c)
				}
				if info.Remaining == 0 {
					return rl.rejectRequest(c, info)
				}
				c.Response().Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", info.Remaining))
			}

			return next(c)
		}
	}
}

// resolveActor determines the rate limit actor.
// 認証済みユーザーのIDを使い、未認証ならIPハッシュを使う。
// enableIPRateLimit=falseかつ未認証の場合は空文字を返す（制限スキップ）。
func (rl *RateLimiter) resolveActor(c echo.Context) string {
	if user := GetUser(c); user != nil {
		return user.ID
	}
	if !rl.enableIPRateLimit {
		return ""
	}
	return ipHash(c.RealIP())
}

// rejectRequest returns a 429 response with appropriate headers.
func (rl *RateLimiter) rejectRequest(c echo.Context, info LimitInfo) error {
	retryAfterSec := (info.ResetMs - time.Now().UnixMilli() + 999) / 1000
	if retryAfterSec < 0 {
		retryAfterSec = 0
	}
	c.Response().Header().Set("Retry-After", fmt.Sprintf("%d", retryAfterSec))
	return c.JSON(http.StatusTooManyRequests, apierr.RateLimitExceeded())
}

// ipHash computes a rate-limit key for an IP address.
// TS版 getIpHash 互換: IPv6は/64マスク、IPv4はフルアドレスをハッシュ。
func ipHash(ipStr string) string {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		// パース不能な場合はSHA256フォールバック
		h := sha256.Sum256([]byte(ipStr))
		n := new(big.Int).SetBytes(h[:8])
		return "ip-" + n.Text(36)
	}

	if ip4 := ip.To4(); ip4 != nil {
		// IPv4: フルアドレスをそのまま使用
		n := new(big.Int).SetBytes(ip4)
		return "ip-" + n.Text(36)
	}

	// IPv6: /64マスク（先頭8バイトのみ）
	ip16 := ip.To16()
	n := new(big.Int).SetBytes(ip16[:8])
	return "ip-" + n.Text(36)
}
