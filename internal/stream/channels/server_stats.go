package channels

import (
	"encoding/json"

	"github.com/shiroha-a/mk/internal/stream"
)

// ServerStatsChannel broadcasts server statistics.
type ServerStatsChannel struct {
	ctx       stream.ChannelContext
	connected bool
}

// NewServerStats returns a channel factory for "serverStats".
func NewServerStats(ctx stream.ChannelContext) stream.Channel {
	return &ServerStatsChannel{ctx: ctx}
}

func (c *ServerStatsChannel) Init(_ json.RawMessage) error {
	c.connected = true
	c.ctx.Subscribe("serverStats")
	return nil
}

func (c *ServerStatsChannel) OnRedisEvent(payload []byte) {
	_ = c.ctx.Send("stats", json.RawMessage(payload))
}

func (c *ServerStatsChannel) OnClientMessage(string, json.RawMessage) {}

func (c *ServerStatsChannel) Dispose() {
	if c.connected {
		c.ctx.Unsubscribe("serverStats")
	}
}
