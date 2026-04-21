package stream

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capturePubSub は Publish 呼び出しをメモリに記録するテスト用 publisher。
type capturePubSub struct {
	mu    sync.Mutex
	calls []struct {
		channel string
		payload []byte
	}
}

func (c *capturePubSub) Publish(_ context.Context, channel string, payload any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	// payload is json.RawMessage which is []byte
	var raw []byte
	if b, ok := payload.(json.RawMessage); ok {
		raw = []byte(b)
	}
	c.calls = append(c.calls, struct {
		channel string
		payload []byte
	}{channel, raw})
	return nil
}

func (c *capturePubSub) snapshot() []struct {
	channel string
	payload []byte
} {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]struct {
		channel string
		payload []byte
	}, len(c.calls))
	copy(out, c.calls)
	return out
}

func TestServerStatsPublisher_PublishesSnapshot(t *testing.T) {
	pub := &capturePubSub{}
	// 短い interval で Start → Stop し、最低 1 回 tick されたことを確認する。
	p := NewServerStatsPublisher(pub, 20*time.Millisecond)
	p.Start()
	time.Sleep(60 * time.Millisecond)
	p.Stop()

	calls := pub.snapshot()
	require.NotEmpty(t, calls, "publisher should tick at least once")
	for _, c := range calls {
		assert.Equal(t, "serverStats", c.channel)
		// payload should parse as serverStatsSnapshot JSON shape.
		var body struct {
			CPU float64 `json:"cpu"`
			Mem struct {
				Used   uint64 `json:"used"`
				Active uint64 `json:"active"`
			} `json:"mem"`
			Net struct {
				Rx uint64 `json:"rx"`
				Tx uint64 `json:"tx"`
			} `json:"net"`
			Fs struct {
				R uint64 `json:"r"`
				W uint64 `json:"w"`
			} `json:"fs"`
		}
		require.NoError(t, json.Unmarshal(c.payload, &body))
	}
}

func TestServerStatsPublisher_DoubleStartNoOp(t *testing.T) {
	pub := &capturePubSub{}
	p := NewServerStatsPublisher(pub, 50*time.Millisecond)
	p.Start()
	p.Start() // second call must be a no-op
	time.Sleep(10 * time.Millisecond)
	p.Stop()
	// Stop を 2 回呼んでも panic しないこと。
	p.Stop()
}

func TestServerStatsPublisher_NilPubNoOp(t *testing.T) {
	// pub が nil でも panic せず、Start はno-opとなる。
	p := NewServerStatsPublisher(nil, 50*time.Millisecond)
	p.Start()
	p.Stop()
}

func TestDiffPerSec(t *testing.T) {
	assert.Equal(t, uint64(100), diffPerSec(200, 100, 1.0))
	assert.Equal(t, uint64(50), diffPerSec(200, 100, 2.0))
	// underflow (counter reset) clamps to 0
	assert.Equal(t, uint64(0), diffPerSec(50, 100, 1.0))
	// zero elapsed clamps to 0
	assert.Equal(t, uint64(0), diffPerSec(200, 100, 0))
}
