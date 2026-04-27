package driver

import "time"

// EnqueueOptions captures driver-neutral parameters for enqueueing a
// single task. Concrete drivers translate these into native options.
//
// Zero-valued fields mean "use the driver default":
//   - Queue ""           : driver decides (often the default queue name)
//   - MaxRetry 0         : driver default retry count
//   - UniqueTTL 0        : no unique-key dedup
//   - ProcessIn 0        : process immediately
type EnqueueOptions struct {
	Queue     string
	MaxRetry  int
	UniqueTTL time.Duration
	ProcessIn time.Duration

	// MaxRetrySet distinguishes "MaxRetry left at default" from
	// "MaxRetry explicitly set to 0". asynq treats MaxRetry=0 as
	// no-retries which differs from "use default", so callers like
	// the cleanRemoteNotes job that want zero retries must opt in
	// explicitly.
	MaxRetrySet bool
}

// EnqueueOption mutates EnqueueOptions; pass via Client.Enqueue.
type EnqueueOption func(*EnqueueOptions)

// WithQueue routes the task to the named queue.
func WithQueue(name string) EnqueueOption {
	return func(o *EnqueueOptions) { o.Queue = name }
}

// WithMaxRetry sets the maximum number of retries for the task. Use
// zero to disable retries entirely (callers should be aware that
// driver defaults differ — asynq defaults to 25).
func WithMaxRetry(n int) EnqueueOption {
	return func(o *EnqueueOptions) {
		o.MaxRetry = n
		o.MaxRetrySet = true
	}
}

// WithUnique sets a uniqueness TTL: the driver suppresses duplicate
// enqueues with the same task type + payload within the window.
func WithUnique(ttl time.Duration) EnqueueOption {
	return func(o *EnqueueOptions) { o.UniqueTTL = ttl }
}

// WithProcessIn delays processing of the task by the given duration.
func WithProcessIn(d time.Duration) EnqueueOption {
	return func(o *EnqueueOptions) { o.ProcessIn = d }
}

// ApplyEnqueueOptions folds the variadic options into a single
// EnqueueOptions. Drivers call this to obtain a populated struct
// before mapping to their native API.
func ApplyEnqueueOptions(opts []EnqueueOption) EnqueueOptions {
	var o EnqueueOptions
	for _, fn := range opts {
		fn(&o)
	}
	return o
}
