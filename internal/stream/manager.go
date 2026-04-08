package stream

import (
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
	"github.com/shiroha-a/mk/internal/model"
)

// Manager owns the set of live streaming connections. Channel registry と
// PubSub bus を握り、各 connection に Dispatcher を割り当てて pubsub →
// channel のルーティングを行う。
type Manager struct {
	mu       sync.RWMutex
	conns    map[string]*Connection
	nextID   atomic.Uint64
	registry *Registry
	bus      PubSubBus
}

// NewManager constructs a Manager with no live connections. registry / bus が
// nil でも動作する (channel framework を一切使わないテスト用)。
func NewManager(registry *Registry, bus PubSubBus) *Manager {
	return &Manager{
		conns:    make(map[string]*Connection),
		registry: registry,
		bus:      bus,
	}
}

// Accept implements api/streaming.ConnectionAcceptor. *websocket.Conn から
// Connection を組み立て、Dispatcher 経由で channel framework に橋渡しする。
func (m *Manager) Accept(ws *websocket.Conn, user *model.User) {
	id := m.allocateID()
	c := NewConnection(id, user, ws)
	dispatcher := NewDispatcher(c, m.registry, m.bus)
	c.SetMessageHandler(dispatcher.HandleClientMessage)
	c.SetCloseHandler(func() {
		dispatcher.CloseAll()
		m.unregister(id)
	})
	m.register(c)
	// Start ブロックするので呼び出し元 goroutine 上で読み続ける。
	c.Start()
}

// Count returns the number of currently registered connections. 主にテストと
// 運用メトリクス用。
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.conns)
}

// Get returns the connection for id, or nil if not registered.
func (m *Manager) Get(id string) *Connection {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.conns[id]
}

// Shutdown closes every registered connection. サーバー停止時に呼ぶ。
func (m *Manager) Shutdown() {
	m.mu.Lock()
	conns := make([]*Connection, 0, len(m.conns))
	for _, c := range m.conns {
		conns = append(conns, c)
	}
	m.conns = map[string]*Connection{}
	m.mu.Unlock()
	for _, c := range conns {
		c.Close()
	}
}

func (m *Manager) register(c *Connection) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.conns[c.ID()] = c
}

func (m *Manager) unregister(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.conns, id)
}

// allocateID returns a fresh sequential id for a new connection. WebSocket は
// プロセスローカルなので uint64 で十分。
func (m *Manager) allocateID() string {
	n := m.nextID.Add(1)
	return formatID(n)
}

// formatID converts the numeric counter to a short hex string.
func formatID(n uint64) string {
	const digits = "0123456789abcdef"
	if n == 0 {
		return "0"
	}
	var buf [16]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = digits[n&0xf]
		n >>= 4
	}
	return string(buf[i:])
}
