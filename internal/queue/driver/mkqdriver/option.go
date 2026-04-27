package mkqdriver

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/shiroha-a/mkq"

	"github.com/shiroha-a/mk/internal/queue/driver"
)

// toMkqAddOptions translates a driver.EnqueueOptions into the
// equivalent []mkq.AddOption list. Unique semantics map to mkq's
// WithUnique (BullMQ deduplication with TTL keyed by taskType +
// queue + payload hash), and MaxRetry maps to WithAttempts (asynq
// exposes MaxRetry=N, mkq exposes Attempts=N+1; we keep the asynq
// number for caller continuity at the cost of one extra attempt —
// matches "MaxRetry=N means up to N retries on top of the first try").
//
// The taskType is needed both as the BullMQ Job.name (so the admin
// UI can identify the task in lists) and as part of the unique key.
// The payload bytes are folded into the unique key via SHA-256 so
// that distinct payloads of the same task type are NOT silently
// collapsed (e.g. two EnqueueDeleteAccount calls for different users
// must both run, not be deduped).
func toMkqAddOptions(o driver.EnqueueOptions, taskType string, payload []byte) []mkq.AddOption {
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
		// asynq.Unique の key は (queue, type, payload) — payload が
		// 違えば別ジョブとして許容される。mkq.WithUnique は明示 ID 必須
		// なので taskType + payload hash を結合した文字列を使う。
		//
		// 直訳ではないが意味的には asynq に揃う:
		//   - cleanRemoteNotes / reactionFlush: payload は nil → hash 同値
		//     → asynq と同じく taskType ベースで dedup
		//   - deleteAccount: payload に UserID を含む → 異なる user の
		//     job は異なる hash → asynq と同じく独立して enqueue される
		//   - chart cron: payload は nil → 上と同様
		//
		// payload は事実上 immutable の前提 (mk-go 内部からの enqueue
		// 経路はすべて構造体を JSON marshal してから渡す) なので、
		// hash 衝突は SHA-256 衝突と同レベルの確率で起きるだけ。
		out = append(out, mkq.WithUnique(uniqueKey(taskType, payload), o.UniqueTTL))
	}
	if o.ProcessIn > 0 {
		out = append(out, mkq.WithDelay(o.ProcessIn))
	}
	return out
}

// uniqueKey builds a stable mkq dedup key from the task type and
// payload bytes. The first 16 hex chars (8 bytes) of SHA-256 are
// enough to keep the BullMQ key short while making collision
// astronomically unlikely for the volumes mk-go realistically sees.
func uniqueKey(taskType string, payload []byte) string {
	h := sha256.Sum256(payload)
	return "queue:" + taskType + ":" + hex.EncodeToString(h[:8])
}
