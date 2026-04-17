package channels

import (
	"encoding/json"

	"github.com/shiroha-a/mk/internal/stream"
)

// QueueStatsChannel broadcasts job queue statistics.
type QueueStatsChannel struct {
	ctx       stream.ChannelContext
	connected bool
}

// NewQueueStats returns a channel factory for "queueStats".
func NewQueueStats(ctx stream.ChannelContext) stream.Channel {
	return &QueueStatsChannel{ctx: ctx}
}

func (c *QueueStatsChannel) Init(_ json.RawMessage) error {
	c.connected = true
	c.ctx.Subscribe("queueStats")
	return nil
}

func (c *QueueStatsChannel) OnRedisEvent(payload []byte) {
	_ = c.ctx.Send("stats", json.RawMessage(payload))
}

// OnClientMessage handles requestLog from the client. TS本家ではstats
// collectorがqueueStatsLog:{id}トピックに応答するrequest-responseパターンを
// 使うが、現状collectorは未実装のため空配列を返す。
func (c *QueueStatsChannel) OnClientMessage(msgType string, body json.RawMessage) {
	if msgType != "requestLog" {
		return
	}
	_ = c.ctx.Send("statsLog", []any{})
}

// ShouldShare implements stream.ShareableChannel.
func (c *QueueStatsChannel) ShouldShare() bool { return true }

func (c *QueueStatsChannel) Dispose() {
	if c.connected {
		c.ctx.Unsubscribe("queueStats")
	}
}
