package driver_test

import (
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/queue/driver"
	"github.com/stretchr/testify/assert"
)

func TestWithBackoff(t *testing.T) {
	o := driver.ApplyEnqueueOptions([]driver.EnqueueOption{
		driver.WithBackoff("fixed", 5*time.Second),
	})
	assert.Equal(t, "fixed", o.BackoffType)
	assert.Equal(t, 5*time.Second, o.BackoffDelay)
}

func TestWithFederationBackoff(t *testing.T) {
	o := driver.ApplyEnqueueOptions([]driver.EnqueueOption{
		driver.WithFederationBackoff(),
	})
	assert.Equal(t, "exponential", o.BackoffType)
	assert.Equal(t, time.Minute, o.BackoffDelay)
}

// 未設定なら backoff フィールドは空のまま (= mkq は即時 retry)。
func TestNoBackoffByDefault(t *testing.T) {
	o := driver.ApplyEnqueueOptions(nil)
	assert.Empty(t, o.BackoffType)
	assert.Zero(t, o.BackoffDelay)
}
