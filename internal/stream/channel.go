package stream

import (
	"encoding/json"
	"sync"
)

// Channel is the per-subscription state for one streaming channel attached to
// a single Connection. Implementations are constructed by a ChannelFactory
// when the client sends a `connect` envelope and torn down on `disconnect` or
// connection close.
type Channel interface {
	// Init is called once after the channel is registered. params は client が
	// connect で渡した任意 JSON で、channel 実装ごとに解釈する。
	Init(params json.RawMessage)

	// OnRedisEvent is called by the Manager when a subscribed Redis pub/sub
	// channel produces a message. payload は Redis から受信した raw bytes。
	OnRedisEvent(payload []byte)

	// OnClientMessage forwards a `ch` envelope sent by the client to this
	// specific channel id. msgType / body は client envelope の `type` /
	// `body`。
	OnClientMessage(msgType string, body json.RawMessage)

	// Dispose is called once when the channel is unregistered. 実装は購読
	// 解除や goroutine 停止をここで行う。
	Dispose()
}

// ChannelContext is what a Channel implementation receives so it can send
// messages back to its owning Connection without holding a direct pointer.
type ChannelContext interface {
	// ID returns the per-channel id supplied by the client.
	ID() string
	// User returns the authenticated user, or nil for anonymous connections.
	User() any
	// Send marshals payload as JSON and queues it as a `channel` envelope to
	// the client.
	Send(msgType string, body any) error
	// Subscribe registers an additional Redis topic. payload はこの channel の
	// OnRedisEvent に届く。
	Subscribe(topic string)
	// Unsubscribe stops listening on a topic.
	Unsubscribe(topic string)
}

// ShareableChannel is an optional interface a Channel can implement to
// signal that a single Connection should not hold multiple instances of the
// same channel name simultaneously. 例: serverStats/queueStats のような
// broadcast チャンネルは 1 接続につき 1 購読で十分。Dispatcher は connect
// 時に既存の同名チャンネルを検出し、2 回目の接続を no-op にする。
type ShareableChannel interface {
	Channel
	// ShouldShare reports whether this channel instance should be shared.
	// true を返すと、同じ Connection 内の同名チャンネルへの追加 connect は
	// 新しいエントリを作らずに無視される。
	ShouldShare() bool
}

// ChannelFactory builds a new Channel for the given context. Channel name は
// Misskey クライアントが connect.body.channel に指定する文字列 (e.g.
// "homeTimeline").
type ChannelFactory func(ctx ChannelContext) Channel

// Registry maps channel name → ChannelFactory. router.go で各 Channel 実装を
// register することで Manager が参照できる。
type Registry struct {
	mu        sync.RWMutex
	factories map[string]ChannelFactory
}

// NewRegistry constructs an empty Registry.
func NewRegistry() *Registry {
	return &Registry{factories: make(map[string]ChannelFactory)}
}

// Register binds name → factory. 同名で複数回 register した場合は最後の登録が
// 有効になる。
func (r *Registry) Register(name string, factory ChannelFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[name] = factory
}

// Lookup returns the factory for name, or nil when name is unknown.
func (r *Registry) Lookup(name string) ChannelFactory {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.factories[name]
}
