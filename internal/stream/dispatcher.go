package stream

import (
	"encoding/json"
	"sync"
)

// PubSubBus is the minimal pubsub interface that Dispatcher needs. core/event.
// PubSubService がこれを満たす。reference counting は Dispatcher 側で行う。
type PubSubBus interface {
	Subscribe(topic string, handler func([]byte))
	Unsubscribe(topic string)
}

// channelEntry holds an active per-connection channel subscription with its
// list of pubsub topics.
type channelEntry struct {
	id      string
	channel Channel
	topics  map[string]struct{}
}

// Dispatcher is the per-Connection state holding registered channels and the
// reverse map "topic → channel id" used to route inbound pubsub events.
//
// 1 つの Connection に対して 1 つの Dispatcher がぶら下がり、複数の Channel
// を保持する。pubsub の global subscription 管理は Manager の Router (K-4)
// に集約するため、Dispatcher は per-channel の subscribe/unsubscribe API を
// 公開して Manager 側がそれを listen する形にする。
type Dispatcher struct {
	conn     *Connection
	registry *Registry
	bus      PubSubBus

	mu       sync.RWMutex
	channels map[string]*channelEntry   // channel id → entry
	topics   map[string]map[string]bool // topic → set of channel ids
}

// NewDispatcher constructs a Dispatcher for the given connection. registry /
// bus が nil の場合、対応する操作は no-op になる (テスト用)。
func NewDispatcher(conn *Connection, registry *Registry, bus PubSubBus) *Dispatcher {
	return &Dispatcher{
		conn:     conn,
		registry: registry,
		bus:      bus,
		channels: make(map[string]*channelEntry),
		topics:   make(map[string]map[string]bool),
	}
}

// HandleClientMessage parses a Misskey-style envelope and forwards it to the
// appropriate handler. 想定する type は connect / disconnect / ch / その他。
func (d *Dispatcher) HandleClientMessage(msgType string, body json.RawMessage) {
	switch msgType {
	case "connect":
		d.handleConnect(body)
	case "disconnect":
		d.handleDisconnect(body)
	case "ch":
		d.handleChannelMessage(body)
	}
}

// handleConnect creates a new Channel via the registry and registers it.
func (d *Dispatcher) handleConnect(body json.RawMessage) {
	var req struct {
		ID      string          `json:"id"`
		Channel string          `json:"channel"`
		Params  json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(body, &req); err != nil || req.ID == "" || req.Channel == "" {
		return
	}
	if d.registry == nil {
		return
	}
	factory := d.registry.Lookup(req.Channel)
	if factory == nil {
		return
	}
	d.mu.Lock()
	if _, exists := d.channels[req.ID]; exists {
		d.mu.Unlock()
		return
	}
	entry := &channelEntry{id: req.ID, topics: map[string]struct{}{}}
	d.channels[req.ID] = entry
	d.mu.Unlock()

	ctx := &channelContext{dispatcher: d, id: req.ID}
	ch := factory(ctx)
	entry.channel = ch
	ch.Init(req.Params)
}

// handleDisconnect tears down a previously-connected channel.
func (d *Dispatcher) handleDisconnect(body json.RawMessage) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &req); err != nil || req.ID == "" {
		return
	}
	d.removeChannel(req.ID)
}

// handleChannelMessage forwards a `ch` envelope to the matching Channel.
func (d *Dispatcher) handleChannelMessage(body json.RawMessage) {
	var req struct {
		ID   string          `json:"id"`
		Type string          `json:"type"`
		Body json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(body, &req); err != nil || req.ID == "" {
		return
	}
	d.mu.RLock()
	entry, ok := d.channels[req.ID]
	d.mu.RUnlock()
	if !ok {
		return
	}
	entry.channel.OnClientMessage(req.Type, req.Body)
}

// removeChannel disposes of a channel entry and unsubscribes from any topics
// it owned (with reference counting).
func (d *Dispatcher) removeChannel(id string) {
	d.mu.Lock()
	entry, ok := d.channels[id]
	if !ok {
		d.mu.Unlock()
		return
	}
	delete(d.channels, id)
	topics := make([]string, 0, len(entry.topics))
	for t := range entry.topics {
		topics = append(topics, t)
		set := d.topics[t]
		delete(set, id)
		if len(set) == 0 {
			delete(d.topics, t)
		}
	}
	d.mu.Unlock()

	for _, t := range topics {
		// このトピックが他の channel からも参照されていなければ pubsub
		// 解除を実行する。
		d.mu.RLock()
		stillUsed := len(d.topics[t]) > 0
		d.mu.RUnlock()
		if !stillUsed && d.bus != nil {
			d.bus.Unsubscribe(t)
		}
	}
	if entry.channel != nil {
		entry.channel.Dispose()
	}
}

// CloseAll tears down every registered channel. Manager が connection close
// 時に呼ぶ。
func (d *Dispatcher) CloseAll() {
	d.mu.Lock()
	ids := make([]string, 0, len(d.channels))
	for id := range d.channels {
		ids = append(ids, id)
	}
	d.mu.Unlock()
	for _, id := range ids {
		d.removeChannel(id)
	}
}

// subscribe registers a topic for a channel id and ensures the bus is listening.
func (d *Dispatcher) subscribe(channelID, topic string) {
	d.mu.Lock()
	entry, ok := d.channels[channelID]
	if !ok {
		d.mu.Unlock()
		return
	}
	if _, dup := entry.topics[topic]; dup {
		d.mu.Unlock()
		return
	}
	entry.topics[topic] = struct{}{}
	set, exists := d.topics[topic]
	if !exists {
		set = map[string]bool{}
		d.topics[topic] = set
	}
	set[channelID] = true
	first := !exists
	d.mu.Unlock()

	if first && d.bus != nil {
		d.bus.Subscribe(topic, func(payload []byte) { d.fanout(topic, payload) })
	}
}

// unsubscribe undoes subscribe for a single channel id.
func (d *Dispatcher) unsubscribe(channelID, topic string) {
	d.mu.Lock()
	entry, ok := d.channels[channelID]
	if !ok {
		d.mu.Unlock()
		return
	}
	if _, present := entry.topics[topic]; !present {
		d.mu.Unlock()
		return
	}
	delete(entry.topics, topic)
	set := d.topics[topic]
	delete(set, channelID)
	last := len(set) == 0
	if last {
		delete(d.topics, topic)
	}
	d.mu.Unlock()

	if last && d.bus != nil {
		d.bus.Unsubscribe(topic)
	}
}

// fanout dispatches a pubsub payload to every channel subscribed to topic.
func (d *Dispatcher) fanout(topic string, payload []byte) {
	d.mu.RLock()
	ids := make([]string, 0, len(d.topics[topic]))
	for id := range d.topics[topic] {
		ids = append(ids, id)
	}
	channels := make([]Channel, 0, len(ids))
	for _, id := range ids {
		if entry, ok := d.channels[id]; ok {
			channels = append(channels, entry.channel)
		}
	}
	d.mu.RUnlock()

	for _, ch := range channels {
		if ch != nil {
			ch.OnRedisEvent(payload)
		}
	}
}

// channelContext is the per-channel facade implementing ChannelContext. それ
// 自身は薄く、実体は Dispatcher 経由で Connection に橋渡しする。
type channelContext struct {
	dispatcher *Dispatcher
	id         string
}

func (c *channelContext) ID() string { return c.id }
func (c *channelContext) User() any {
	if c.dispatcher.conn == nil {
		return nil
	}
	return c.dispatcher.conn.User()
}

// Send wraps the payload in a Misskey "channel" envelope and queues it on the
// underlying Connection.
func (c *channelContext) Send(msgType string, body any) error {
	if c.dispatcher.conn == nil {
		return nil
	}
	return c.dispatcher.conn.Send(map[string]any{
		"type": "channel",
		"body": map[string]any{
			"id":   c.id,
			"type": msgType,
			"body": body,
		},
	})
}

func (c *channelContext) Subscribe(topic string)   { c.dispatcher.subscribe(c.id, topic) }
func (c *channelContext) Unsubscribe(topic string) { c.dispatcher.unsubscribe(c.id, topic) }
