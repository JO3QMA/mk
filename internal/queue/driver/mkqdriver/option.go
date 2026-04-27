package mkqdriver

import (
	"github.com/shiroha-a/mkq"

	"github.com/shiroha-a/mk/internal/queue/driver"
)

// toMkqAddOptions translates a driver.EnqueueOptions into the
// equivalent []mkq.AddOption list. Unique semantics map to mkq's
// WithUnique (BullMQ deduplication with TTL keyed by taskType +
// queue), and MaxRetry maps to WithAttempts (asynq exposes
// MaxRetry=N, mkq exposes Attempts=N+1; we keep the asynq number for
// caller continuity at the cost of one extra attempt — matches
// "MaxRetry=N means up to N retries on top of the first try").
//
// The taskType is needed both as the BullMQ Job.name (so the admin
// UI can identify the task in lists) and as the unique key prefix.
func toMkqAddOptions(o driver.EnqueueOptions, taskType string) []mkq.AddOption {
	out := make([]mkq.AddOption, 0, 4)
	if taskType != "" {
		out = append(out, mkq.WithJobName(taskType))
	}
	if o.MaxRetrySet {
		// asynq の MaxRetry(N) は「初回 + N 回 retry」= 合計 N+1 attempts。
		// mkq の WithAttempts は合計 attempts なので +1 する。
		out = append(out, mkq.WithAttempts(o.MaxRetry+1))
	}
	if o.UniqueTTL > 0 {
		// asynq.Unique は (queue, type, payload) を key にする。
		// mkq の WithUnique は明示 ID 必須なので taskType を ID として
		// 使う。これにより同一 (queue, taskType) の重複 enqueue が
		// TTL 内で抑止される。payload を絡めない分 asynq よりやや広く
		// 弾くが、mk-go で WithUnique を使う 4 ケース (cleanRemoteNotes /
		// reactionFlush / deleteAccount / cron) はすべて payload なし
		// または payload-uniform なので実害なし。
		out = append(out, mkq.WithUnique("queue:"+taskType, o.UniqueTTL))
	}
	if o.ProcessIn > 0 {
		out = append(out, mkq.WithDelay(o.ProcessIn))
	}
	return out
}
