package cache

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/misskey-dev/misskey-go/internal/config"
	"github.com/redis/go-redis/v9"
)

// RedisClients holds multiple Redis connections for different purposes.
type RedisClients struct {
	Default   *redis.Client
	Pubsub    *redis.Client
	JobQueue  *redis.Client
	Timelines *redis.Client
	Reactions *redis.Client
}

// NewRedisClients creates Redis clients from configuration.
func NewRedisClients(cfg *config.Config) (*RedisClients, error) {
	clients := &RedisClients{
		Default:   newClient(cfg.Redis),
		Pubsub:    newClient(cfg.RedisForPubsub),
		JobQueue:  newClient(cfg.RedisForJobQueue),
		Timelines: newClient(cfg.RedisForTimelines),
		Reactions: newClient(cfg.RedisForReactions),
	}

	ctx := context.Background()
	if err := clients.Default.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	slog.Info("connected to Redis",
		"host", cfg.Redis.Host,
		"port", cfg.Redis.Port,
	)

	return clients, nil
}

func newClient(opts config.RedisOptions) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", opts.Host, opts.Port),
		Password: opts.Pass,
		DB:       opts.DB,
		Username: opts.Username,
	})
}

// Close closes all Redis connections.
func (r *RedisClients) Close() error {
	for _, c := range []*redis.Client{
		r.Default, r.Pubsub, r.JobQueue, r.Timelines, r.Reactions,
	} {
		if err := c.Close(); err != nil {
			return err
		}
	}
	return nil
}

// KeyPrefix returns the key prefix for Redis keys.
func KeyPrefix(cfg *config.Config) string {
	return cfg.Redis.Prefix + ":"
}
