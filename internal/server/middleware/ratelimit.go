package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
)

// RateLimitConfig defines rate limit parameters.
type RateLimitConfig struct {
	Duration time.Duration
	Max      int
}

// RateLimiter provides IP-based rate limiting.
type RateLimiter struct {
	mu       sync.Mutex
	counters map[string]*rateLimitEntry
}

type rateLimitEntry struct {
	count   int
	resetAt time.Time
}

// NewRateLimiter creates a new RateLimiter.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		counters: make(map[string]*rateLimitEntry),
	}
}

// Limit returns an Echo middleware that enforces rate limiting.
func (rl *RateLimiter) Limit(cfg RateLimitConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			key := fmt.Sprintf("%s:%s", c.RealIP(), c.Path())

			rl.mu.Lock()
			entry, exists := rl.counters[key]
			now := time.Now()

			if !exists || now.After(entry.resetAt) {
				entry = &rateLimitEntry{
					count:   0,
					resetAt: now.Add(cfg.Duration),
				}
				rl.counters[key] = entry
			}

			entry.count++
			remaining := cfg.Max - entry.count
			rl.mu.Unlock()

			if remaining < 0 {
				retryAfter := time.Until(entry.resetAt).Seconds()
				c.Response().Header().Set("Retry-After", fmt.Sprintf("%.0f", retryAfter))
				return c.JSON(http.StatusTooManyRequests, map[string]any{
					"error": map[string]any{
						"message": "Rate limit exceeded. Please try again later.",
						"code":    "RATE_LIMIT_EXCEEDED",
						"id":      "d5826d14-3982-4d2e-8011-b9e9f02499ef",
					},
				})
			}

			c.Response().Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
			return next(c)
		}
	}
}
