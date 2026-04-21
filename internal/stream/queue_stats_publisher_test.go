package stream

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubQueueInspector serves canned QueueStatsInfo for one queue name.
type stubQueueInspector struct {
	info *QueueStatsInfo
	err  error
	hits int
}

func (s *stubQueueInspector) GetQueueInfo(_ string) (*QueueStatsInfo, error) {
	s.hits++
	if s.err != nil {
		return nil, s.err
	}
	return s.info, nil
}

func TestQueueStatsPublisher_PublishesDeliverDepths(t *testing.T) {
	pub := &capturePubSub{}
	insp := &stubQueueInspector{info: &QueueStatsInfo{Active: 3, Pending: 7, Retry: 1}}
	p := NewQueueStatsPublisher(insp, pub, 30*time.Millisecond)
	p.Start()
	time.Sleep(60 * time.Millisecond)
	p.Stop()

	calls := pub.snapshot()
	require.NotEmpty(t, calls)
	for _, c := range calls {
		assert.Equal(t, "queueStats", c.channel)
		var body struct {
			Deliver struct {
				Active              int `json:"active"`
				Waiting             int `json:"waiting"`
				Delayed             int `json:"delayed"`
				ActiveSincePrevTick int `json:"activeSincePrevTick"`
			} `json:"deliver"`
			Inbox struct {
				Active int `json:"active"`
			} `json:"inbox"`
		}
		require.NoError(t, json.Unmarshal(c.payload, &body))
		assert.Equal(t, 3, body.Deliver.Active)
		assert.Equal(t, 7, body.Deliver.Waiting)
		assert.Equal(t, 1, body.Deliver.Delayed)
		// inbox is a compatibility zero stub (mk-go has no inbox queue).
		assert.Equal(t, 0, body.Inbox.Active)
	}
}

func TestQueueStatsPublisher_InspectorErrorPublishesZeros(t *testing.T) {
	pub := &capturePubSub{}
	insp := &stubQueueInspector{err: errors.New("boom")}
	p := NewQueueStatsPublisher(insp, pub, 30*time.Millisecond)
	p.Start()
	time.Sleep(50 * time.Millisecond)
	p.Stop()

	calls := pub.snapshot()
	require.NotEmpty(t, calls)
	var body struct {
		Deliver struct {
			Active int `json:"active"`
		} `json:"deliver"`
	}
	require.NoError(t, json.Unmarshal(calls[0].payload, &body))
	assert.Equal(t, 0, body.Deliver.Active)
}

func TestQueueStatsPublisher_NilInspectorNoOp(t *testing.T) {
	pub := &capturePubSub{}
	p := NewQueueStatsPublisher(nil, pub, 30*time.Millisecond)
	p.Start()
	time.Sleep(20 * time.Millisecond)
	p.Stop()
	assert.Empty(t, pub.snapshot())
}

func TestQueueStatsPublisher_DoubleStopSafe(t *testing.T) {
	pub := &capturePubSub{}
	insp := &stubQueueInspector{info: &QueueStatsInfo{}}
	p := NewQueueStatsPublisher(insp, pub, 50*time.Millisecond)
	p.Start()
	p.Stop()
	p.Stop() // no panic
}
