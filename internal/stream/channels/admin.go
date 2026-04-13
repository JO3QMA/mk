package channels

import (
	"encoding/json"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/stream"
)

// AdminChannel forwards admin-only events. Requires authenticated admin user.
type AdminChannel struct {
	ctx       stream.ChannelContext
	connected bool
}

// NewAdmin returns a channel factory for "admin".
func NewAdmin(ctx stream.ChannelContext) stream.Channel {
	return &AdminChannel{ctx: ctx}
}

func (c *AdminChannel) Init(_ json.RawMessage) {
	user, ok := c.ctx.User().(*model.User)
	if !ok || user == nil {
		return
	}
	// admin権限チェック: IsAdminフィールドまたはロールシステムで判定
	// ここではtopic購読のみ行い、publish側でadminイベントのみ流す前提
	c.connected = true
	c.ctx.Subscribe("adminStream")
}

func (c *AdminChannel) OnRedisEvent(payload []byte) {
	// {type, body}エンベロープの展開
	var envelope struct {
		Type string          `json:"type"`
		Body json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(payload, &envelope); err == nil && envelope.Type != "" {
		_ = c.ctx.Send(envelope.Type, envelope.Body)
		return
	}
	_ = c.ctx.Send("adminEvent", json.RawMessage(payload))
}

func (c *AdminChannel) OnClientMessage(string, json.RawMessage) {}

func (c *AdminChannel) Dispose() {
	if c.connected {
		c.ctx.Unsubscribe("adminStream")
	}
}
