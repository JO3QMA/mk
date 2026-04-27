package driver

import "errors"

// SkipRetry is a sentinel returned by a HandlerFunc to tell the driver
// the job must not be retried even if attempts remain. Drivers map
// this to their native skip-retry semantics (asynq.SkipRetry,
// mkq.ErrUnrecoverable, etc.).
//
// Handlers typically wrap it with fmt.Errorf("%w: %w", err, driver.SkipRetry)
// so callers can both inspect the underlying cause and observe the skip
// signal via errors.Is.
var SkipRetry = errors.New("queue driver: skip retry")
