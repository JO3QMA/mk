package cache

import (
	"context"
	"log"
	"os"
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

func newTestCacheService() *CacheService {
	clients := &RedisClients{Default: testRedis.Client}
	return NewCacheService(clients, "test:")
}

func TestCacheService_SetAndGet(t *testing.T) {
	svc := newTestCacheService()
	ctx := context.Background()
	testRedis.FlushAll(ctx)

	type payload struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	err := svc.Set(ctx, "key1", payload{Name: "test", Value: 42}, time.Minute)
	require.NoError(t, err)

	var got payload
	found, err := svc.Get(ctx, "key1", &got)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "test", got.Name)
	assert.Equal(t, 42, got.Value)
}

func TestCacheService_Get_NotFound(t *testing.T) {
	svc := newTestCacheService()
	ctx := context.Background()
	testRedis.FlushAll(ctx)

	var got string
	found, err := svc.Get(ctx, "nonexistent", &got)
	require.NoError(t, err)
	assert.False(t, found)
}

func TestCacheService_Delete(t *testing.T) {
	svc := newTestCacheService()
	ctx := context.Background()
	testRedis.FlushAll(ctx)

	require.NoError(t, svc.Set(ctx, "delme", "value", time.Minute))

	require.NoError(t, svc.Delete(ctx, "delme"))

	var got string
	found, _ := svc.Get(ctx, "delme", &got)
	assert.False(t, found)
}

func TestCacheService_Exists(t *testing.T) {
	svc := newTestCacheService()
	ctx := context.Background()
	testRedis.FlushAll(ctx)

	exists, err := svc.Exists(ctx, "nope")
	require.NoError(t, err)
	assert.False(t, exists)

	require.NoError(t, svc.Set(ctx, "yep", "val", time.Minute))

	exists, err = svc.Exists(ctx, "yep")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestCacheService_TTLExpiry(t *testing.T) {
	svc := newTestCacheService()
	ctx := context.Background()
	testRedis.FlushAll(ctx)

	require.NoError(t, svc.Set(ctx, "expiring", "val", 100*time.Millisecond))

	var got string
	found, _ := svc.Get(ctx, "expiring", &got)
	assert.True(t, found)

	time.Sleep(150 * time.Millisecond)

	found, _ = svc.Get(ctx, "expiring", &got)
	assert.False(t, found)
}
