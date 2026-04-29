package queue

// Policy captures runtime tuning knobs for a single logical queue. The
// fields are applied lazily by NewClient (default MaxRetry on enqueue) and
// by the driver Server constructors (worker concurrency / rate limit). All
// zero values mean "use the driver default" — silent no-op when unset.
//
// MaxAttempts only affects enqueue paths that pre-pend the default before
// caller opts (currently EnqueueDeliver only — webhook / cleanRemoteNotes
// / reactionFlush keep their hard-coded retry policies because they encode
// task-specific semantics rather than queue-wide tuning).
type Policy struct {
	// Concurrency overrides the worker pool size for this queue. 0 means
	// "fall back to driver default" — for asynq this is the global pool
	// gated by priority weights, for mkq it is total/len(queues).
	Concurrency int

	// RatePerSec caps task processing throughput at N tasks per second.
	// 0 means no limit. Implemented as a token-bucket inside the worker
	// dispatch path, so it back-pressures handler invocations rather
	// than rejecting enqueues.
	RatePerSec int

	// MaxAttempts is the default `WithMaxRetry` applied at enqueue time
	// when the caller didn't pass `WithMaxRetry` explicitly. 0 means
	// "fall back to driver default".
	MaxAttempts int
}

// PolicyMap maps queue name → Policy. Lookups for missing queues return
// the zero Policy, which the driver / client treats as "no override".
type PolicyMap map[string]Policy

// PolicyFor returns the Policy registered for queueName, or the zero value
// when nothing is configured. Safe to call on a nil PolicyMap.
func (m PolicyMap) PolicyFor(queueName string) Policy {
	if m == nil {
		return Policy{}
	}
	return m[queueName]
}
