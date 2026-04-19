package cache

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"
	"github.com/shiroha-a/mk/internal/config"
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

	if config.IsUnixSocketPath(cfg.Redis.Host) {
		slog.Info("connected to Redis", "socket", cfg.Redis.Host)
	} else {
		slog.Info("connected to Redis",
			"host", cfg.Redis.Host,
			"port", cfg.Redis.Port,
		)
	}

	return clients, nil
}

func newClient(opts config.RedisOptions) *redis.Client {
	return redis.NewClient(buildRedisOptions(opts))
}

// buildRedisOptions maps mk-go の config.RedisOptions を go-redis の
// redis.Options に変換する。Host が UNIX domain socket パス ("/" 始まり) の
// ときは Network を "unix" に切り替え、Addr にはパスをそのまま入れる。
// そうでなければ従来どおり host:port 形式の TCP 接続にする。
func buildRedisOptions(opts config.RedisOptions) *redis.Options {
	// go-redisのデフォルトPoolSizeは runtime.GOMAXPROCS * 10。
	// 明示的に指定された場合のみ上書きする（0はgo-redisデフォルトを意味する）。
	poolSize := 0
	if opts.PoolSize != nil {
		poolSize = *opts.PoolSize
	}

	if config.IsUnixSocketPath(opts.Host) {
		return &redis.Options{
			Network:  "unix",
			Addr:     opts.Host,
			Password: opts.Pass,
			DB:       opts.DB,
			Username: opts.Username,
			PoolSize: poolSize,
		}
	}
	return &redis.Options{
		Addr:     fmt.Sprintf("%s:%d", opts.Host, opts.Port),
		Password: opts.Pass,
		DB:       opts.DB,
		Username: opts.Username,
		PoolSize: poolSize,
	}
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
