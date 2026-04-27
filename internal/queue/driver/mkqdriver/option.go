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
		// **意図的な広めの dedup**: asynq.Unique の key は
		// (queue, type, payload) なので「同じ payload の重複だけ」
		// を弾く。mkq.WithUnique は明示 ID 必須なので taskType を
		// 使い、(queue, taskType) で弾く。結果として mkq driver は
		// payload が違っても同一 taskType を TTL 内で全弾きする —
		// asynq driver より広い。
		//
		// mk-go で WithUnique を呼ぶ 4 経路 (cleanRemoteNotes /
		// reactionFlush / deleteAccount / cron) はすべて payload
		// なし or payload-uniform なので意味的差分はゼロ。将来
		// payload-distinct な uniqueness を要求する経路を追加する
		// 場合は taskType + hash(payload) のような key 構築に拡張
		// する必要があり、ここを update する。
		out = append(out, mkq.WithUnique("queue:"+taskType, o.UniqueTTL))
	}
	if o.ProcessIn > 0 {
		out = append(out, mkq.WithDelay(o.ProcessIn))
	}
	return out
}
