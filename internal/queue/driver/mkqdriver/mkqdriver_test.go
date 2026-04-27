package mkqdriver

import (
	"testing"
	"time"

	"github.com/shiroha-a/mkq"

	"github.com/shiroha-a/mk/internal/config"
	"github.com/shiroha-a/mk/internal/queue/driver"
)

func TestBuildRedisOptions_TCP(t *testing.T) {
	got := BuildRedisOptions(config.RedisOptions{
		Host: "redis.example",
		Port: 16379,
		Pass: "p",
		DB:   3,
	})
	if len(got.Addrs) != 1 || got.Addrs[0] != "redis.example:16379" {
		t.Fatalf("Addrs: got %v", got.Addrs)
	}
	if got.Password != "p" || got.DB != 3 {
		t.Fatalf("Auth fields: got %+v", got)
	}
}

func TestBuildRedisOptions_UnixSocket(t *testing.T) {
	got := BuildRedisOptions(config.RedisOptions{
		Host: "/tmp/redis.sock",
		Pass: "p",
	})
	if len(got.Addrs) != 1 || got.Addrs[0] != "/tmp/redis.sock" {
		t.Fatalf("Addrs: got %v", got.Addrs)
	}
}

func TestToMkqAddOptions_FullSet(t *testing.T) {
	o := driver.ApplyEnqueueOptions([]driver.EnqueueOption{
		driver.WithQueue("deliver"),
		driver.WithMaxRetry(4),
		driver.WithUnique(time.Hour),
		driver.WithProcessIn(2 * time.Second),
	})
	got := toMkqAddOptions(o, "ap:deliver")
	if len(got) < 4 {
		// 4 options expected: WithJobName, WithAttempts, WithUnique, WithDelay
		t.Fatalf("expected at least 4 options, got %d", len(got))
	}
}

func TestToMkqAddOptions_DefaultsSkipped(t *testing.T) {
	got := toMkqAddOptions(driver.EnqueueOptions{}, "")
	if len(got) != 0 {
		t.Fatalf("expected zero options for default EnqueueOptions, got %d", len(got))
	}
}

func TestToMkqAddOptions_JobNameOnly(t *testing.T) {
	got := toMkqAddOptions(driver.EnqueueOptions{}, "ap:deliver")
	if len(got) != 1 {
		// Just WithJobName.
		t.Fatalf("expected 1 option, got %d", len(got))
	}
}

func TestToMkqAddOptions_MaxRetryRequiresExplicit(t *testing.T) {
	// MaxRetrySet=false → default attempts; no option emitted.
	got := toMkqAddOptions(driver.EnqueueOptions{MaxRetry: 3}, "task")
	// 1 option only (WithJobName); MaxRetry must NOT be added.
	if len(got) != 1 {
		t.Fatalf("MaxRetry without MaxRetrySet must be skipped, got %d", len(got))
	}
}

func TestMkqTask_Wrapper(t *testing.T) {
	w := mkqTask{taskType: "x", payload: []byte("body")}
	if w.Type() != "x" {
		t.Fatalf("Type: got %q", w.Type())
	}
	if string(w.Payload()) != "body" {
		t.Fatalf("Payload: got %q", string(w.Payload()))
	}
}

func TestBuildRedisOptions_PoolSize(t *testing.T) {
	pool := 64
	got := BuildRedisOptions(config.RedisOptions{
		Host:     "redis.example",
		Port:     6379,
		PoolSize: &pool,
	})
	if got.PoolSize != 64 {
		t.Fatalf("PoolSize: want 64, got %d", got.PoolSize)
	}
}

func TestBuildRedisOptions_PoolSizeNilKeepsDefault(t *testing.T) {
	got := BuildRedisOptions(config.RedisOptions{
		Host: "redis.example",
		Port: 6379,
	})
	if got.PoolSize != 0 {
		// 0 = "use go-redis default"
		t.Fatalf("PoolSize: want 0 (default), got %d", got.PoolSize)
	}
}

func TestBuildRedisOptions_PoolSizeZeroIgnored(t *testing.T) {
	pool := 0
	got := BuildRedisOptions(config.RedisOptions{
		Host:     "redis.example",
		Port:     6379,
		PoolSize: &pool,
	})
	if got.PoolSize != 0 {
		t.Fatalf("PoolSize: want 0, got %d", got.PoolSize)
	}
}

func TestJobToSummary_Nil(t *testing.T) {
	if got := jobToSummary("q", "wait", nil, nil); got != nil {
		t.Fatalf("nil job must yield nil summary, got %#v", got)
	}
}

func TestJobToSummary_FramedPayload(t *testing.T) {
	job := &mkq.Job[framedPayload]{
		ID:           "1",
		Name:         "ap:deliver",
		Data:         framedPayload{Type: "ap:deliver", Body: []byte("body")},
		Timestamp:    time.Unix(1700000000, 0),
		AttemptsMade: 2,
	}
	state := &mkq.JobState{
		ProcessedOn:  time.Unix(1700000010, 0),
		FinishedOn:   time.Unix(1700000020, 0),
		FailedReason: "boom",
	}
	got := jobToSummary("deliver", "wait", job, state)
	if got == nil {
		t.Fatal("got nil")
	}
	if got.Type != "ap:deliver" {
		t.Fatalf("Type: got %q", got.Type)
	}
	if got.Retried != 2 {
		t.Fatalf("Retried: got %d", got.Retried)
	}
	if string(got.Payload) != "body" {
		t.Fatalf("Payload: got %q", got.Payload)
	}
	if got.LastErr != "boom" {
		t.Fatalf("LastErr: got %q", got.LastErr)
	}
	if got.LastFailedAt != state.FinishedOn {
		t.Fatalf("LastFailedAt: got %v", got.LastFailedAt)
	}
	if got.CompletedAt != state.FinishedOn {
		t.Fatalf("CompletedAt: got %v", got.CompletedAt)
	}
	if got.NextProcessAt != state.ProcessedOn {
		t.Fatalf("NextProcessAt: got %v", got.NextProcessAt)
	}
}

func TestJobToSummary_TypeFallbackToJobName(t *testing.T) {
	// Foreign jobs (created by BullMQ TS without our framing) have an
	// empty Data.Type. The summary must fall back to Job.Name so admin
	// UIs at least see a non-blank label.
	job := &mkq.Job[framedPayload]{
		ID:   "1",
		Name: "foreign:type",
		Data: framedPayload{}, // no Type
	}
	got := jobToSummary("deliver", "wait", job, nil)
	if got.Type != "foreign:type" {
		t.Fatalf("Type fallback: got %q", got.Type)
	}
}

func TestJobToSummary_NilState(t *testing.T) {
	job := &mkq.Job[framedPayload]{ID: "1", Data: framedPayload{Type: "x"}}
	got := jobToSummary("q", "wait", job, nil)
	if got == nil {
		t.Fatal("got nil")
	}
	if !got.LastFailedAt.IsZero() {
		t.Fatalf("LastFailedAt should be zero, got %v", got.LastFailedAt)
	}
}

func TestClient_CloseIsNoop(t *testing.T) {
	// Driver-level Close handles the underlying *mkq.Client; per the
	// driver split, queue.Client.Close from the layered API forwards
	// to mkqdriver.Client.Close which must be a clean no-op.
	c := &Client{driver: nil}
	if err := c.Close(); err != nil {
		t.Fatalf("Client.Close: %v", err)
	}
}

func TestInspector_CloseIsNoop(t *testing.T) {
	// Same contract as Client.Close — the underlying client is owned
	// by Driver.
	i := &Inspector{driver: nil}
	if err := i.Close(); err != nil {
		t.Fatalf("Inspector.Close: %v", err)
	}
}
