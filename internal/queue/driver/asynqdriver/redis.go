// Package asynqdriver implements queue/driver against the asynq
// (github.com/hibiken/asynq) library. It is the historical default
// driver for mk-go and preserves the exact retry/scheduling
// semantics callers were used to before the driver abstraction
// landed.
package asynqdriver

import (
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/shiroha-a/mk/internal/config"
)

// BuildRedisOpt converts mk-go's config.RedisOptions into an
// asynq.RedisClientOpt. Hosts that look like a UNIX socket path
// ("/" prefix) flip the Network to "unix"; all other hosts use
// classic host:port TCP.
//
// PoolSize is propagated when the operator set redisForJobQueue.poolSize
// in YAML (asynq forwards it to the underlying go-redis client). Nil
// keeps asynq's default behaviour. The mkq driver propagates the same
// field via redis.UniversalOptions.PoolSize so both drivers honour
// the YAML setting symmetrically.
//
// 旧 server.buildAsynqRedisOpt をそのまま移植したもので、admin/queue
// stats の挙動を含めて従来と完全に同じ接続を生成する。
func BuildRedisOpt(opts config.RedisOptions) asynq.RedisClientOpt {
	out := asynq.RedisClientOpt{
		Password: opts.Pass,
		DB:       opts.DB,
		Username: opts.Username,
	}
	if config.IsUnixSocketPath(opts.Host) {
		out.Network = "unix"
		out.Addr = opts.Host
	} else {
		out.Addr = fmt.Sprintf("%s:%d", opts.Host, opts.Port)
	}
	if opts.PoolSize != nil && *opts.PoolSize > 0 {
		out.PoolSize = *opts.PoolSize
	}
	return out
}
