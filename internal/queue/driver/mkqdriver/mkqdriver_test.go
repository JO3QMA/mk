package mkqdriver

import (
	"testing"
	"time"

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
