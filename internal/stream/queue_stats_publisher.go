package stream

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"
)

// QueueInspector is the minimal subset of queue.Inspector needed for emitting
// streaming stats. interfaceで受け取ることでテストでstub化できる。
type QueueInspector interface {
	GetQueueInfo(qname string) (*QueueStatsInfo, error)
}

// QueueStatsInfo mirrors the relevant fields from queue.InspectorInfo so that
// this package does not have to import internal/queue. 循環依存防止。
// Delayed は Bull 用語で asynq の Scheduled + Retry 合計に対応するので
// 両方の field を保持する。
type QueueStatsInfo struct {
	Active    int
	Pending   int
	Scheduled int
	Retry     int
}

// QueueStatsPublisher periodically queries asynq queue depths and publishes
// them to the `queueStats` PubSub topic for the admin dashboard / widget.
//
// 本家Misskey QueueStatsServiceは `deliver` と `inbox` の2キーを出すが、
// mk-goのinbox処理はHTTP同期で asynq queue を持たない。そのため inbox は
// 互換性のためにゼロ固定で出力し、`deliver` には実数を入れる。他のasynq
// queue (push / webhook等) は出さない (frontendが見ていないため)。
type QueueStatsPublisher struct {
	inspector QueueInspector
	pub       PubSubPublisher
	interval  time.Duration
	stopCh    chan struct{}
	wg        sync.WaitGroup
	started   bool
	mu        sync.Mutex

	// log / logMax: server_stats と同等の ring buffer (新しい順、最大
	// QueueStatsLogMax 件)。requestLog 応答用。
	logMu  sync.Mutex
	log    []json.RawMessage
	logMax int
}

// QueueStatsLogMax は requestLog 応答で返す最大件数。TS 本家と同じ 200。
const QueueStatsLogMax = 200

// NewQueueStatsPublisher constructs a QueueStatsPublisher. interval<=0 は
// デフォルト (3秒) を使用する。
func NewQueueStatsPublisher(inspector QueueInspector, pub PubSubPublisher, interval time.Duration) *QueueStatsPublisher {
	if interval <= 0 {
		interval = 3 * time.Second
	}
	return &QueueStatsPublisher{
		inspector: inspector,
		pub:       pub,
		interval:  interval,
		stopCh:    make(chan struct{}),
		logMax:    QueueStatsLogMax,
	}
}

// Start begins the collection loop.
func (p *QueueStatsPublisher) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.started || p.pub == nil || p.inspector == nil {
		return
	}
	p.started = true
	p.wg.Add(1)
	go p.loop()
}

// Stop signals the loop to exit and waits for completion.
func (p *QueueStatsPublisher) Stop() {
	p.mu.Lock()
	if !p.started {
		p.mu.Unlock()
		return
	}
	close(p.stopCh)
	p.started = false
	p.mu.Unlock()
	p.wg.Wait()
}

func (p *QueueStatsPublisher) loop() {
	defer p.wg.Done()
	p.tick()
	t := time.NewTicker(p.interval)
	defer t.Stop()
	for {
		select {
		case <-p.stopCh:
			return
		case <-t.C:
			p.tick()
		}
	}
}

// queueStatsBody matches the upstream { deliver, inbox } schema so that the
// frontend admin/overview.queue chart accepts our payload directly.
type queueStatsBody struct {
	Deliver queueStatsEntry `json:"deliver"`
	Inbox   queueStatsEntry `json:"inbox"`
}

type queueStatsEntry struct {
	// ActiveSincePrevTick はBullのactive eventカウントに相当。asynqには
	// 同等APIがないため、とりあえず現active数を出す (本家ほど厳密でなくて
	// もグラフは描画できる)。
	ActiveSincePrevTick int `json:"activeSincePrevTick"`
	Active              int `json:"active"`
	Waiting             int `json:"waiting"`
	Delayed             int `json:"delayed"`
}

func (p *QueueStatsPublisher) tick() {
	body := queueStatsBody{
		Deliver: p.deliverEntry(),
		// inboxは mk-go では queue を持たないので全て0固定 (frontend互換)。
		Inbox: queueStatsEntry{},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		slog.Warn("queue stats: marshal failed", "err", err)
		return
	}
	rawMsg := json.RawMessage(raw)
	p.appendLog(rawMsg)
	if err := p.pub.Publish(context.Background(), "queueStats", rawMsg); err != nil {
		slog.Warn("queue stats: publish failed", "err", err)
	}
}

// appendLog は server_stats と同じく ring buffer に最新を unshift する。
func (p *QueueStatsPublisher) appendLog(raw json.RawMessage) {
	p.logMu.Lock()
	defer p.logMu.Unlock()
	copied := append(json.RawMessage(nil), raw...)
	p.log = append([]json.RawMessage{copied}, p.log...)
	if len(p.log) > p.logMax {
		p.log = p.log[:p.logMax]
	}
}

// Log returns the most recent up to maxLen snapshots, newest first. server_stats
// の Log と同じ semantics。
func (p *QueueStatsPublisher) Log(maxLen int) []json.RawMessage {
	if maxLen <= 0 || maxLen > QueueStatsLogMax {
		maxLen = QueueStatsLogMax
	}
	p.logMu.Lock()
	defer p.logMu.Unlock()
	n := len(p.log)
	if n > maxLen {
		n = maxLen
	}
	out := make([]json.RawMessage, n)
	copy(out, p.log[:n])
	return out
}

// deliverQueueName は本番のasynq queue名。テストで差し替えられるように
// 変数化する。
var deliverQueueName = "deliver"

func (p *QueueStatsPublisher) deliverEntry() queueStatsEntry {
	info, err := p.inspector.GetQueueInfo(deliverQueueName)
	if err != nil || info == nil {
		return queueStatsEntry{}
	}
	return queueStatsEntry{
		ActiveSincePrevTick: info.Active,
		Active:              info.Active,
		Waiting:             info.Pending,
		// Bull の delayed は asynq の Scheduled (未来実行予定) と Retry
		// (失敗後再試行待ち) の両方を含む。
		Delayed: info.Scheduled + info.Retry,
	}
}
