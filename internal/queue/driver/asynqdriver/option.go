package asynqdriver

import (
	"github.com/hibiken/asynq"

	"github.com/shiroha-a/mk/internal/queue/driver"
)

// toAsynqOptions translates a driver.EnqueueOptions into the
// equivalent []asynq.Option list. Empty / zero fields are skipped so
// the asynq library applies its own defaults (e.g. MaxRetry=25).
func toAsynqOptions(o driver.EnqueueOptions) []asynq.Option {
	out := make([]asynq.Option, 0, 4)
	if o.Queue != "" {
		out = append(out, asynq.Queue(o.Queue))
	}
	if o.MaxRetrySet {
		// 0 を明示的に指定するケース (cleanRemoteNotes / chart 等の
		// retry 不要ジョブ) があるため driver 側の MaxRetrySet で
		// 「default 値ではない」ことを判定する。
		out = append(out, asynq.MaxRetry(o.MaxRetry))
	}
	if o.UniqueTTL > 0 {
		out = append(out, asynq.Unique(o.UniqueTTL))
	}
	if o.ProcessIn > 0 {
		out = append(out, asynq.ProcessIn(o.ProcessIn))
	}
	return out
}
