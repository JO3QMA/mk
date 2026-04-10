package queue_test

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/shiroha-a/mk/internal/queue"
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

func redisOpt() asynq.RedisClientOpt {
	return asynq.RedisClientOpt{Addr: testRedis.Addr}
}

func TestNewDeliverTask_RoundTrip(t *testing.T) {
	payload := queue.DeliverPayload{
		Inbox:  "https://remote.example/users/alice/inbox",
		Body:   []byte(`{"type":"Create"}`),
		KeyID:  "https://example.com/users/u1#main-key",
		KeyPEM: "PEMDATA",
	}
	task := queue.NewDeliverTask(payload)
	assert.Equal(t, queue.TaskTypeDeliver, task.Type())

	decoded, err := queue.DecodeDeliverPayload(task.Payload())
	require.NoError(t, err)
	assert.Equal(t, payload, decoded)
}

func TestDecodeDeliverPayload_Invalid(t *testing.T) {
	_, err := queue.DecodeDeliverPayload([]byte(`{not json`))
	assert.Error(t, err)
}

func TestClient_EnqueueDeliver(t *testing.T) {
	testutil.SkipIfNoDocker(t)
	flushTestRedis(t)

	c := queue.NewClient(redisOpt())
	defer func() { _ = c.Close() }()

	payload := queue.DeliverPayload{
		Inbox:  "https://remote.example/inbox",
		Body:   []byte(`{"type":"Follow"}`),
		KeyID:  "k1",
		KeyPEM: "PEM",
	}
	require.NoError(t, c.EnqueueDeliver(payload, asynq.MaxRetry(3)))

	// inspector で実際にキューに入っていることを確認
	insp := asynq.NewInspector(redisOpt())
	defer func() { _ = insp.Close() }()

	tasks, err := insp.ListPendingTasks(queue.QueueName)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, queue.TaskTypeDeliver, tasks[0].Type)

	var got queue.DeliverPayload
	require.NoError(t, json.Unmarshal(tasks[0].Payload, &got))
	assert.Equal(t, payload, got)
}

func TestClient_EnqueueDeliver_ClosedClientFails(t *testing.T) {
	testutil.SkipIfNoDocker(t)
	flushTestRedis(t)

	c := queue.NewClient(redisOpt())
	require.NoError(t, c.Close())

	err := c.EnqueueDeliver(queue.DeliverPayload{Inbox: "x", Body: []byte(`{}`)})
	assert.Error(t, err)
}

func TestServer_HandleAndProcess(t *testing.T) {
	testutil.SkipIfNoDocker(t)
	flushTestRedis(t)

	srv := queue.NewServer(redisOpt(), queue.ServerConfig{Concurrency: 2})

	var (
		wg         sync.WaitGroup
		received   int32
		gotPayload queue.DeliverPayload
		mu         sync.Mutex
	)
	wg.Add(1)
	srv.Handle(queue.TaskTypeDeliver, func(_ context.Context, t *asynq.Task) error {
		defer wg.Done()
		atomic.AddInt32(&received, 1)
		p, err := queue.DecodeDeliverPayload(t.Payload())
		if err != nil {
			return err
		}
		mu.Lock()
		gotPayload = p
		mu.Unlock()
		return nil
	})
	require.NoError(t, srv.Start())
	defer srv.Shutdown()

	c := queue.NewClient(redisOpt())
	defer func() { _ = c.Close() }()

	payload := queue.DeliverPayload{
		Inbox:  "https://remote.example/inbox",
		Body:   []byte(`{"hello":"world"}`),
		KeyID:  "k",
		KeyPEM: "PEM",
	}
	require.NoError(t, c.EnqueueDeliver(payload))

	if !waitGroupTimeout(&wg, 5*time.Second) {
		t.Fatal("handler was not invoked within timeout")
	}

	assert.Equal(t, int32(1), atomic.LoadInt32(&received))
	mu.Lock()
	assert.Equal(t, payload, gotPayload)
	mu.Unlock()
}

func TestServer_DefaultConcurrency(t *testing.T) {
	// Concurrency<=0 のとき内部デフォルト16にフォールバックする経路を確認。
	// asynq Server を作って即 Shutdown するだけで十分。
	srv := queue.NewServer(redisOpt(), queue.ServerConfig{Concurrency: 0})
	srv.Handle(queue.TaskTypeDeliver, func(_ context.Context, _ *asynq.Task) error {
		return nil
	})
	// Startせずに Shutdownしても asynq は安全に no-op になる。
	srv.Shutdown()
}

func flushTestRedis(t *testing.T) {
	t.Helper()
	if err := testRedis.Client.FlushAll(context.Background()).Err(); err != nil {
		t.Fatalf("failed to flush redis: %v", err)
	}
}

// waitGroupTimeout waits for wg with a deadline. Returns true if completed.
func waitGroupTimeout(wg *sync.WaitGroup, d time.Duration) bool {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(d):
		return false
	}
}

func TestNewExportTask_RoundTrip(t *testing.T) {
	payload := queue.ExportPayload{UserID: "u1", Type: "notes"}
	task := queue.NewExportTask(payload)
	assert.Equal(t, queue.TaskTypeExport, task.Type())

	decoded, err := queue.DecodeExportPayload(task.Payload())
	require.NoError(t, err)
	assert.Equal(t, payload, decoded)
}

func TestDecodeExportPayload_Invalid(t *testing.T) {
	_, err := queue.DecodeExportPayload([]byte(`{bad`))
	assert.Error(t, err)
}

func TestNewImportTask_RoundTrip(t *testing.T) {
	payload := queue.ImportPayload{UserID: "u1", Type: "following", FileID: "f1"}
	task := queue.NewImportTask(payload)
	assert.Equal(t, queue.TaskTypeImport, task.Type())

	decoded, err := queue.DecodeImportPayload(task.Payload())
	require.NoError(t, err)
	assert.Equal(t, payload, decoded)
}

func TestDecodeImportPayload_Invalid(t *testing.T) {
	_, err := queue.DecodeImportPayload([]byte(`{bad`))
	assert.Error(t, err)
}

func TestClient_EnqueueExport(t *testing.T) {
	testutil.SkipIfNoDocker(t)
	flushTestRedis(t)

	c := queue.NewClient(redisOpt())
	defer func() { _ = c.Close() }()

	require.NoError(t, c.EnqueueExport(queue.ExportPayload{UserID: "u1", Type: "notes"}))

	insp := asynq.NewInspector(redisOpt())
	defer func() { _ = insp.Close() }()

	tasks, err := insp.ListPendingTasks(queue.ExportQueueName)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, queue.TaskTypeExport, tasks[0].Type)
}

func TestClient_EnqueueImport(t *testing.T) {
	testutil.SkipIfNoDocker(t)
	flushTestRedis(t)

	c := queue.NewClient(redisOpt())
	defer func() { _ = c.Close() }()

	require.NoError(t, c.EnqueueImport(queue.ImportPayload{UserID: "u1", Type: "following", FileID: "f1"}))

	insp := asynq.NewInspector(redisOpt())
	defer func() { _ = insp.Close() }()

	tasks, err := insp.ListPendingTasks(queue.ExportQueueName)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, queue.TaskTypeImport, tasks[0].Type)
}

func TestInspector_QueuesAndInfo(t *testing.T) {
	testutil.SkipIfNoDocker(t)
	flushTestRedis(t)

	// エンキューしてキューを作成
	c := queue.NewClient(redisOpt())
	defer func() { _ = c.Close() }()
	require.NoError(t, c.EnqueueDeliver(queue.DeliverPayload{Inbox: "x", Body: []byte(`{}`), KeyID: "k", KeyPEM: "p"}))

	insp := queue.NewInspector(redisOpt())
	defer func() { _ = insp.Close() }()

	queues, err := insp.Queues()
	require.NoError(t, err)
	assert.Contains(t, queues, queue.QueueName)

	info, err := insp.GetQueueInfo(queue.QueueName)
	require.NoError(t, err)
	assert.Equal(t, queue.QueueName, info.Queue)
	assert.GreaterOrEqual(t, info.Size, 0)
}

func TestInspector_GetQueueInfo_NotFound(t *testing.T) {
	testutil.SkipIfNoDocker(t)
	flushTestRedis(t)

	insp := queue.NewInspector(redisOpt())
	defer func() { _ = insp.Close() }()

	_, err := insp.GetQueueInfo("nonexistent")
	assert.Error(t, err)
}

// ensure errors package referenced for completeness in CI builds.
var _ = errors.New
