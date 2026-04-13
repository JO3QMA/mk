package channels

import (
	"encoding/json"

	"github.com/shiroha-a/mk/internal/stream"
)

// ChannelTimelineChannel forwards notes posted to a specific channel.
type ChannelTimelineChannel struct {
	ctx    stream.ChannelContext
	topic  string
	filter noteFilter
}

// NewChannelTimeline returns a channel factory for "channel".
func NewChannelTimeline(ctx stream.ChannelContext) stream.Channel {
	return &ChannelTimelineChannel{ctx: ctx}
}

func (c *ChannelTimelineChannel) Init(params json.RawMessage) {
	var p struct {
		ChannelID string `json:"channelId"`
	}
	if len(params) > 0 {
		_ = json.Unmarshal(params, &p)
	}
	if p.ChannelID == "" {
		return
	}
	c.filter = parseNoteFilter(params)
	c.topic = "channel:" + p.ChannelID
	c.ctx.Subscribe(c.topic)
}

func (c *ChannelTimelineChannel) OnRedisEvent(payload []byte) {
	if !c.filter.shouldEmit(payload) {
		return
	}
	_ = c.ctx.Send("note", json.RawMessage(payload))
}

func (c *ChannelTimelineChannel) OnClientMessage(string, json.RawMessage) {}

func (c *ChannelTimelineChannel) Dispose() {
	if c.topic != "" {
		c.ctx.Unsubscribe(c.topic)
	}
}
