package mkqdriver

import (
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/shiroha-a/mk/internal/config"
)

// BuildRedisOptions converts mk-go's config.RedisOptions into the
// redis.UniversalOptions value mkq.Config consumes. UNIX socket paths
// (any host string starting with "/") are passed through unchanged —
// go-redis auto-detects the network type from the leading slash.
func BuildRedisOptions(opts config.RedisOptions) redis.UniversalOptions {
	addr := opts.Host
	if !config.IsUnixSocketPath(opts.Host) {
		addr = fmt.Sprintf("%s:%d", opts.Host, opts.Port)
	}
	return redis.UniversalOptions{
		Addrs:    []string{addr},
		Password: opts.Pass,
		Username: opts.Username,
		DB:       opts.DB,
	}
}
