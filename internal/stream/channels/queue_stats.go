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

func (c *QueueStatsChannel) Init(_ json.RawMessage) {
	c.connected = true
	c.ctx.Subscribe("queueStats")
}

func (c *QueueStatsChannel) OnRedisEvent(payload []byte) {
	_ = c.ctx.Send("stats", json.RawMessage(payload))
}

func (c *QueueStatsChannel) OnClientMessage(string, json.RawMessage) {}

func (c *QueueStatsChannel) Dispose() {
	if c.connected {
		c.ctx.Unsubscribe("queueStats")
	}
}
