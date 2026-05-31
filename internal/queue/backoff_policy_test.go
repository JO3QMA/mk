package queue

import (
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/queue/driver"
	"github.com/stretchr/testify/assert"
)

func TestBackoffOptFromPolicy(t *testing.T) {
	// 未指定なら既定の exponential base 1s。
	def := driver.ApplyEnqueueOptions([]driver.EnqueueOption{backoffOptFromPolicy(Policy{})})
	assert.Equal(t, "exponential", def.BackoffType)
	assert.Equal(t, time.Minute, def.BackoffDelay)

	// Policy で上書きできる。
	override := driver.ApplyEnqueueOptions([]driver.EnqueueOption{
		backoffOptFromPolicy(Policy{BackoffType: "fixed", BackoffDelay: 10 * time.Second}),
	})
	assert.Equal(t, "fixed", override.BackoffType)
	assert.Equal(t, 10*time.Second, override.BackoffDelay)

	// delay 0 は不完全な指定として既定にフォールバックする。
	partial := driver.ApplyEnqueueOptions([]driver.EnqueueOption{
		backoffOptFromPolicy(Policy{BackoffType: "fixed"}),
	})
	assert.Equal(t, "exponential", partial.BackoffType)
	assert.Equal(t, time.Minute, partial.BackoffDelay)
}
