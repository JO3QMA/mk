package channels

import (
	"encoding/json"

	"github.com/shiroha-a/mk/internal/stream"
)

// RoleTimelineChannel forwards notes from users with a specific role.
type RoleTimelineChannel struct {
	ctx    stream.ChannelContext
	topic  string
	filter noteFilter
}

// NewRoleTimeline returns a channel factory for "roleTimeline".
func NewRoleTimeline(ctx stream.ChannelContext) stream.Channel {
	return &RoleTimelineChannel{ctx: ctx}
}

func (c *RoleTimelineChannel) Init(params json.RawMessage) {
	var p struct {
		RoleID string `json:"roleId"`
	}
	if len(params) > 0 {
		_ = json.Unmarshal(params, &p)
	}
	if p.RoleID == "" {
		return
	}
	c.filter = parseNoteFilter(params)
	c.topic = "roleTimeline:" + p.RoleID
	c.ctx.Subscribe(c.topic)
}

func (c *RoleTimelineChannel) OnRedisEvent(payload []byte) {
	if !c.filter.shouldEmit(payload) {
		return
	}
	_ = c.ctx.Send("note", json.RawMessage(payload))
}

func (c *RoleTimelineChannel) OnClientMessage(string, json.RawMessage) {}

func (c *RoleTimelineChannel) Dispose() {
	if c.topic != "" {
		c.ctx.Unsubscribe(c.topic)
	}
}
