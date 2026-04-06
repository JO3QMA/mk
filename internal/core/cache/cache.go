package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// CacheService provides key-value caching backed by Redis.
type CacheService struct {
	client *redis.Client
	prefix string
}

// NewCacheService creates a new CacheService.
func NewCacheService(clients *RedisClients, prefix string) *CacheService {
	return &CacheService{
		client: clients.Default,
		prefix: prefix,
	}
}

func (c *CacheService) key(k string) string {
	return c.prefix + k
}

// Get retrieves a value from cache and unmarshals it into dest.
// Returns false if the key does not exist.
func (c *CacheService) Get(ctx context.Context, key string, dest any) (bool, error) {
	val, err := c.client.Get(ctx, c.key(key)).Bytes()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("cache get: %w", err)
	}
	if err := json.Unmarshal(val, dest); err != nil {
		return false, fmt.Errorf("cache unmarshal: %w", err)
	}
	return true, nil
}

// Set stores a value in cache with the given TTL.
func (c *CacheService) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache marshal: %w", err)
	}
	return c.client.Set(ctx, c.key(key), data, ttl).Err()
}

// Delete removes a key from cache.
func (c *CacheService) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, c.key(key)).Err()
}

// Exists checks if a key exists in cache.
func (c *CacheService) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.client.Exists(ctx, c.key(key)).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
