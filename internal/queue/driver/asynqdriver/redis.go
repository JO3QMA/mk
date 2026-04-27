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
// 旧 server.buildAsynqRedisOpt をそのまま移植したもので、admin/queue
// stats の挙動を含めて従来と完全に同じ接続を生成する。
func BuildRedisOpt(opts config.RedisOptions) asynq.RedisClientOpt {
	if config.IsUnixSocketPath(opts.Host) {
		return asynq.RedisClientOpt{
			Network:  "unix",
			Addr:     opts.Host,
			Password: opts.Pass,
			DB:       opts.DB,
			Username: opts.Username,
		}
	}
	return asynq.RedisClientOpt{
		Addr:     fmt.Sprintf("%s:%d", opts.Host, opts.Port),
		Password: opts.Pass,
		DB:       opts.DB,
		Username: opts.Username,
	}
}
