package reversi

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// FederationIDCache manages federationId ↔ gameId mapping in Redis.
type FederationIDCache struct {
	redis redis.Cmdable
}

// NewFederationIDCache creates a new cache.
func NewFederationIDCache(r redis.Cmdable) *FederationIDCache {
	return &FederationIDCache{redis: r}
}

// Set stores a federationId → gameId mapping.
func (c *FederationIDCache) Set(ctx context.Context, federationID, gameID string) {
	if c.redis == nil {
		return
	}
	c.redis.Set(ctx, "reversi:federationId:"+federationID, gameID, 5*time.Minute)
}

// Get retrieves a gameId from a federationId.
func (c *FederationIDCache) Get(ctx context.Context, federationID string) (string, error) {
	if c.redis == nil {
		return "", redis.Nil
	}
	return c.redis.Get(ctx, "reversi:federationId:"+federationID).Result()
}

// ValidUpdateKeys lists the keys that can be changed via federation Update.
var ValidUpdateKeys = map[string]bool{
	"map":                  true,
	"bw":                   true,
	"isLlotheo":            true,
	"canPutEverywhere":     true,
	"loopedBoard":          true,
	"timeLimitForEachTurn": true,
	"noIrregularRules":     true,
}

// IsValidUpdateKey checks if a key is allowed for federation settings updates.
func IsValidUpdateKey(key string) bool {
	return ValidUpdateKeys[key]
}
