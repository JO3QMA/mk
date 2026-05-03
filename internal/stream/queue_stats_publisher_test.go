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

func TestQueueStatsPublisher_PublishesDeliverAndInboxDepths(t *testing.T) {
	// stub は queue 名で値を切り替えられるようにする (deliver / inbox で
	// 同じ inspector を共有するため)。
	infos := map[string]*QueueStatsInfo{
		"deliver": {Active: 3, Pending: 7, Scheduled: 2, Retry: 1},
		"inbox":   {Active: 5, Pending: 11, Scheduled: 0, Retry: 4},
	}
	insp := &perQueueInspector{infos: infos}
	pub := &capturePubSub{}
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
				Active  int `json:"active"`
				Waiting int `json:"waiting"`
				Delayed int `json:"delayed"`
			} `json:"inbox"`
		}
		require.NoError(t, json.Unmarshal(c.payload, &body))
		assert.Equal(t, 3, body.Deliver.Active)
		assert.Equal(t, 7, body.Deliver.Waiting)
		// Bull delayed = asynq Scheduled + Retry
		assert.Equal(t, 3, body.Deliver.Delayed)
		// inbox は #534 で asynq queue 化、#565 で worker 化されたので
		// 実数を出す (#654)。
		assert.Equal(t, 5, body.Inbox.Active)
		assert.Equal(t, 11, body.Inbox.Waiting)
		assert.Equal(t, 4, body.Inbox.Delayed)
	}
}

// perQueueInspector returns a different QueueStatsInfo per queue name so
// tests can verify deliver / inbox are queried independently.
type perQueueInspector struct {
	infos map[string]*QueueStatsInfo
}

func (s *perQueueInspector) GetQueueInfo(qname string) (*QueueStatsInfo, error) {
	return s.infos[qname], nil
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

// publisher が ring buffer に snapshot を貯めて Log() で返せること (#571 item 2)
func TestQueueStatsPublisher_LogReturnsRecentSnapshots(t *testing.T) {
	insp := &stubQueueInspector{info: &QueueStatsInfo{Active: 1, Pending: 2, Scheduled: 3}}
	pub := &capturePubSub{}
	p := NewQueueStatsPublisher(insp, pub, 10*time.Millisecond)
	p.Start()
	time.Sleep(50 * time.Millisecond)
	p.Stop()

	logs := p.Log(0)
	require.NotEmpty(t, logs, "Log should accumulate snapshots from each tick")
	for _, raw := range logs {
		var body map[string]any
		require.NoError(t, json.Unmarshal(raw, &body))
		assert.Contains(t, body, "deliver")
		assert.Contains(t, body, "inbox")
	}
}

// Log(maxLen) で件数制限
func TestQueueStatsPublisher_LogRespectsMaxLen(t *testing.T) {
	insp := &stubQueueInspector{info: &QueueStatsInfo{}}
	pub := &capturePubSub{}
	p := NewQueueStatsPublisher(insp, pub, 5*time.Millisecond)
	p.Start()
	time.Sleep(50 * time.Millisecond)
	p.Stop()

	logs := p.Log(2)
	assert.LessOrEqual(t, len(logs), 2)
}

// ring buffer は QueueStatsLogMax 件で頭打ち
func TestQueueStatsPublisher_LogCapAtMax(t *testing.T) {
	insp := &stubQueueInspector{info: &QueueStatsInfo{}}
	pub := &capturePubSub{}
	p := NewQueueStatsPublisher(insp, pub, 0)
	for i := 0; i < 250; i++ {
		p.logBuf.Append(json.RawMessage(`{}`))
	}
	assert.Equal(t, QueueStatsLogMax, len(p.Log(0)))
}
