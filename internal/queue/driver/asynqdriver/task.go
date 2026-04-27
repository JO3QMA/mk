package asynqdriver

import (
	"github.com/hibiken/asynq"
)

// asynqTask wraps *asynq.Task to satisfy driver.Task. It preserves
// the original asynq.Task pointer so callers that need to escape
// back into asynq-specific behavior can type-assert if absolutely
// required (none currently do).
type asynqTask struct {
	t *asynq.Task
}

// Type returns the task's type identifier.
func (a asynqTask) Type() string { return a.t.Type() }

// Payload returns the task's payload bytes.
func (a asynqTask) Payload() []byte { return a.t.Payload() }
