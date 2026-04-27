package asynqdriver

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/hibiken/asynq"

	"github.com/shiroha-a/mk/internal/config"
	"github.com/shiroha-a/mk/internal/queue/driver"
)

func TestBuildRedisOpt_TCP(t *testing.T) {
	got := BuildRedisOpt(config.RedisOptions{
		Host: "redis.example",
		Port: 16379,
		Pass: "p",
		DB:   3,
	})
	want := asynq.RedisClientOpt{
		Addr:     "redis.example:16379",
		Password: "p",
		DB:       3,
	}
	if got != want {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestBuildRedisOpt_UnixSocket(t *testing.T) {
	got := BuildRedisOpt(config.RedisOptions{
		Host: "/tmp/redis.sock",
		Pass: "p",
	})
	want := asynq.RedisClientOpt{
		Network:  "unix",
		Addr:     "/tmp/redis.sock",
		Password: "p",
	}
	if got != want {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestToAsynqOptions(t *testing.T) {
	o := driver.ApplyEnqueueOptions([]driver.EnqueueOption{
		driver.WithQueue("deliver"),
		driver.WithMaxRetry(0),
		driver.WithUnique(time.Hour),
		driver.WithProcessIn(2 * time.Second),
	})
	got := toAsynqOptions(o)
	if len(got) != 4 {
		t.Fatalf("expected 4 options, got %d (%+v)", len(got), got)
	}
}

func TestToAsynqOptions_DefaultsSkipped(t *testing.T) {
	got := toAsynqOptions(driver.EnqueueOptions{})
	if len(got) != 0 {
		t.Fatalf("expected no options for zero EnqueueOptions, got %d", len(got))
	}
}

func TestToAsynqOptions_MaxRetryRequiresExplicit(t *testing.T) {
	// MaxRetrySet=false means "use driver default" so the option
	// must NOT be added even though MaxRetry==0.
	got := toAsynqOptions(driver.EnqueueOptions{MaxRetry: 5})
	if len(got) != 0 {
		t.Fatalf("MaxRetry without MaxRetrySet must be skipped, got %d", len(got))
	}
}

// fakeRedis returns a non-routable RedisClientOpt; we never Start the
// server in these tests so the address is fine. asynq client/inspector
// constructors do not Dial until the first request.
func fakeRedis() asynq.RedisClientOpt {
	return asynq.RedisClientOpt{Addr: "127.0.0.1:0"}
}

func TestDriver_LazyConstruction(t *testing.T) {
	d := New(fakeRedis(), ServerConfig{Concurrency: 4})

	if d.client != nil || d.server != nil || d.inspector != nil || d.scheduler != nil {
		t.Fatalf("Driver fields must be nil before first access")
	}

	if d.Client() == nil {
		t.Fatal("Client must be non-nil")
	}
	if d.client == nil {
		t.Fatal("Client field must be cached after access")
	}

	if d.Server() == nil {
		t.Fatal("Server must be non-nil")
	}
	if d.Inspector() == nil {
		t.Fatal("Inspector must be non-nil")
	}
	if d.Scheduler() == nil {
		t.Fatal("Scheduler must be non-nil")
	}

	// 2nd call must return the cached instance.
	first := d.Client()
	second := d.Client()
	if first != second {
		t.Fatal("Client must be cached")
	}
}

func TestDriver_Close_NoComponents(t *testing.T) {
	d := New(fakeRedis(), ServerConfig{})
	if err := d.Close(); err != nil {
		t.Fatalf("Close on unused Driver: %v", err)
	}
}

func TestDriver_Close_ClosesConstructedComponents(t *testing.T) {
	d := New(fakeRedis(), ServerConfig{})
	_ = d.Client()
	_ = d.Inspector()
	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestServer_HandleSkipRetryConversion(t *testing.T) {
	s := NewServer(fakeRedis(), ServerConfig{})
	wrapped := fmt.Errorf("decode: %w", driver.SkipRetry)

	var captured error
	s.Handle("dummy", func(_ context.Context, _ driver.Task) error {
		return wrapped
	})

	// Pull the registered handler out of the mux and invoke it
	// directly to verify the SkipRetry conversion. asynq's ServeMux
	// exposes the handler via Handler(*asynq.Task) but only when a
	// task type matches; we synthesize the task here.
	t.Helper()
	h, _ := s.mux.Handler(asynq.NewTask("dummy", nil))
	if h == nil {
		t.Fatal("handler not registered")
	}
	captured = h.ProcessTask(context.Background(), asynq.NewTask("dummy", nil))
	if !errors.Is(captured, asynq.SkipRetry) {
		t.Fatalf("captured error must wrap asynq.SkipRetry, got %v", captured)
	}
	if !errors.Is(captured, driver.SkipRetry) {
		t.Fatalf("captured error must still wrap driver.SkipRetry, got %v", captured)
	}
}

func TestServer_HandlePassesNonSkipErrorThrough(t *testing.T) {
	s := NewServer(fakeRedis(), ServerConfig{})
	want := errors.New("transient")
	s.Handle("plain", func(_ context.Context, _ driver.Task) error {
		return want
	})

	h, _ := s.mux.Handler(asynq.NewTask("plain", nil))
	got := h.ProcessTask(context.Background(), asynq.NewTask("plain", nil))
	if !errors.Is(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	if errors.Is(got, asynq.SkipRetry) {
		t.Fatal("non-SkipRetry handler error must not be tagged with asynq.SkipRetry")
	}
}

func TestServer_DefaultQueueWeights(t *testing.T) {
	// Queues=nil → default fillup applies.
	s := NewServer(fakeRedis(), ServerConfig{Queues: nil})
	if s.inner == nil {
		t.Fatal("inner asynq.Server not constructed")
	}
}

func TestNewServer_ConcurrencyDefault(t *testing.T) {
	// Concurrency<=0 falls back to 16 (covered indirectly — verify
	// constructor does not panic and produces an inner server).
	s := NewServer(fakeRedis(), ServerConfig{})
	if s.inner == nil || s.mux == nil {
		t.Fatal("Server must be fully constructed")
	}
}

func TestAsynqTask_Wrapper(t *testing.T) {
	t1 := asynq.NewTask("foo", []byte("bar"))
	w := asynqTask{t: t1}
	if w.Type() != "foo" {
		t.Fatalf("Type: got %q", w.Type())
	}
	if string(w.Payload()) != "bar" {
		t.Fatalf("Payload: got %q", string(w.Payload()))
	}
}
