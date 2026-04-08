package channels

import (
	"encoding/json"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/stream"
)

// NotificationsChannel forwards notifications addressed to the authenticated
// user. Anonymous connections subscribe to nothing (the channel exists but
// stays silent).
type NotificationsChannel struct {
	ctx   stream.ChannelContext
	topic string
}

// NewNotifications returns a channel factory for "notifications".
func NewNotifications(ctx stream.ChannelContext) stream.Channel {
	return &NotificationsChannel{ctx: ctx}
}

func (c *NotificationsChannel) Init(_ json.RawMessage) {
	user, ok := c.ctx.User().(*model.User)
	if !ok || user == nil {
		return
	}
	c.topic = "notifications:" + user.ID
	c.ctx.Subscribe(c.topic)
}

func (c *NotificationsChannel) OnRedisEvent(payload []byte) {
	_ = c.ctx.Send("notification", json.RawMessage(payload))
}

func (c *NotificationsChannel) OnClientMessage(string, json.RawMessage) {}

func (c *NotificationsChannel) Dispose() {
	if c.topic != "" {
		c.ctx.Unsubscribe(c.topic)
	}
}

// MainChannel is the catch-all per-user channel. It currently forwards both
// notifications and follow events that target the authenticated user. Each
// inbound payload arrives with a hint type so the client can route it.
//
// Misskey 本家の main は notification / mention / unreadMessagingMessage /
// follow / unfollow / followed / receiveFollowRequest / readAllNotifications
// など多種のイベントを受信する。Phase 4.1 では notification と follow event
// に絞る。
type MainChannel struct {
	ctx     stream.ChannelContext
	notif   string
	mainTop string
}

// NewMain returns a channel factory for "main".
func NewMain(ctx stream.ChannelContext) stream.Channel {
	return &MainChannel{ctx: ctx}
}

func (c *MainChannel) Init(_ json.RawMessage) {
	user, ok := c.ctx.User().(*model.User)
	if !ok || user == nil {
		return
	}
	c.notif = "notifications:" + user.ID
	c.mainTop = "main:" + user.ID
	c.ctx.Subscribe(c.notif)
	c.ctx.Subscribe(c.mainTop)
}

// OnRedisEvent attempts to read a hint type from the payload (a JSON object
// with a `type` field) and forwards it under that type. Falls back to
// "unreadNotification" when there is no embedded type field, which keeps
// notification payloads usable by clients that follow the Misskey schema.
func (c *MainChannel) OnRedisEvent(payload []byte) {
	var env struct {
		Type string          `json:"type"`
		Body json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(payload, &env); err == nil && env.Type != "" {
		_ = c.ctx.Send(env.Type, env.Body)
		return
	}
	_ = c.ctx.Send("notification", json.RawMessage(payload))
}

func (c *MainChannel) OnClientMessage(string, json.RawMessage) {}

func (c *MainChannel) Dispose() {
	if c.notif != "" {
		c.ctx.Unsubscribe(c.notif)
	}
	if c.mainTop != "" {
		c.ctx.Unsubscribe(c.mainTop)
	}
}
