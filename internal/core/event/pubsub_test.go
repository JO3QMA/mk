package event

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testRedis *testutil.TestRedis

func TestMain(m *testing.M) {
	ctx := context.Background()
	var err error
	testRedis, err = testutil.SetupRedis(ctx)
	if err != nil {
		log.Fatalf("failed to setup redis: %v", err)
	}

	code := m.Run()

	testRedis.Teardown(ctx)
	os.Exit(code)
}

func TestPubSubService_PublishSubscribe(t *testing.T) {
	svc := NewPubSubService(testRedis.Client, "test:")
	ctx := context.Background()
	defer svc.Close()

	type msg struct {
		Text string `json:"text"`
	}

	var mu sync.Mutex
	var received []msg

	svc.Subscribe(ctx, "chan1", func(data []byte) {
		var m msg
		if err := json.Unmarshal(data, &m); err == nil {
			mu.Lock()
			received = append(received, m)
			mu.Unlock()
		}
	})

	// サブスクリプションが確立されるのを待つ
	time.Sleep(100 * time.Millisecond)

	require.NoError(t, svc.Publish(ctx, "chan1", msg{Text: "hello"}))
	require.NoError(t, svc.Publish(ctx, "chan1", msg{Text: "world"}))

	// メッセージの受信を待つ
	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(received) >= 2
	}, 2*time.Second, 50*time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, "hello", received[0].Text)
	assert.Equal(t, "world", received[1].Text)
}

func TestPubSubService_Unsubscribe(t *testing.T) {
	svc := NewPubSubService(testRedis.Client, "test:")
	ctx := context.Background()
	defer svc.Close()

	var count int
	var mu sync.Mutex

	svc.Subscribe(ctx, "chan2", func(data []byte) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	time.Sleep(100 * time.Millisecond)

	require.NoError(t, svc.Unsubscribe("chan2"))

	// Unsubscribe後のメッセージは受信されない
	time.Sleep(50 * time.Millisecond)
	_ = svc.Publish(ctx, "chan2", "after_unsub")

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	assert.Equal(t, 0, count)
	mu.Unlock()
}

func TestPubSubService_MultipleChannels(t *testing.T) {
	svc := NewPubSubService(testRedis.Client, "test:")
	ctx := context.Background()
	defer svc.Close()

	var mu sync.Mutex
	results := make(map[string]string)

	svc.Subscribe(ctx, "a", func(data []byte) {
		mu.Lock()
		results["a"] = string(data)
		mu.Unlock()
	})
	svc.Subscribe(ctx, "b", func(data []byte) {
		mu.Lock()
		results["b"] = string(data)
		mu.Unlock()
	})

	time.Sleep(100 * time.Millisecond)

	require.NoError(t, svc.Publish(ctx, "a", "msg_a"))
	require.NoError(t, svc.Publish(ctx, "b", "msg_b"))

	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(results) >= 2
	}, 2*time.Second, 50*time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	assert.Contains(t, results["a"], "msg_a")
	assert.Contains(t, results["b"], "msg_b")
}

func TestPubSubService_Unsubscribe_NotSubscribed(t *testing.T) {
	svc := NewPubSubService(testRedis.Client, "test:")
	defer svc.Close()

	// 未登録チャンネルのUnsubscribeはエラーなし
	err := svc.Unsubscribe("nonexistent")
	assert.NoError(t, err)
}

func TestPubSubService_Close_WithSubscriptions(t *testing.T) {
	svc := NewPubSubService(testRedis.Client, "test:")
	ctx := context.Background()

	svc.Subscribe(ctx, "close1", func(data []byte) {})
	svc.Subscribe(ctx, "close2", func(data []byte) {})

	time.Sleep(100 * time.Millisecond)

	err := svc.Close()
	assert.NoError(t, err)
}

func TestPubSubService_Close_AlreadyClosed(t *testing.T) {
	svc := NewPubSubService(testRedis.Client, "test:")
	ctx := context.Background()

	svc.Subscribe(ctx, "doublecl", func(data []byte) {})
	time.Sleep(100 * time.Millisecond)

	// 内部のPubSubを手動で閉じてからCloseを呼ぶ→sub.Close()がエラーを返す
	svc.mu.Lock()
	for _, sub := range svc.subs {
		sub.Close()
	}
	svc.mu.Unlock()

	// svc.Close()は内部でsub.Close()エラーをログに出すが、nilを返す
	err := svc.Close()
	assert.NoError(t, err)
}

func TestPubSubService_Publish_MarshalError(t *testing.T) {
	svc := NewPubSubService(testRedis.Client, "test:")
	ctx := context.Background()
	defer svc.Close()

	// chanがJSONにmarshalできない
	err := svc.Publish(ctx, "ch", make(chan int))
	assert.Error(t, err)
}
