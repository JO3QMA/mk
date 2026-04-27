package mkqdriver

// mkqTask satisfies driver.Task by holding the task type and payload
// captured at dispatch time. The wrapping is necessary because mkq's
// Job[T] holds a typed payload (framedPayload here) rather than the
// raw (type, body) tuple drivers expose.
type mkqTask struct {
	taskType string
	payload  []byte
}

// Type returns the task type identifier.
func (t mkqTask) Type() string { return t.taskType }

// Payload returns the inner task payload bytes.
func (t mkqTask) Payload() []byte { return t.payload }
