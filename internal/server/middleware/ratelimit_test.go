package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mock store ---

type mockCall struct {
	Key      string
	Duration time.Duration
	Max      int
}

type mockResult struct {
	Info LimitInfo
	Err  error
}

type mockLimitStore struct {
	calls   []mockCall
	results []mockResult
	idx     int
}

func (m *mockLimitStore) Check(_ context.Context, key string, duration time.Duration, max int) (LimitInfo, error) {
	m.calls = append(m.calls, mockCall{Key: key, Duration: duration, Max: max})
	i := m.idx
	m.idx++
	if i < len(m.results) {
		return m.results[i].Info, m.results[i].Err
	}
	return LimitInfo{Remaining: 999}, nil
}

// --- helpers ---

func setupEcho(rl *RateLimiter) (*echo.Echo, echo.HandlerFunc) {
	e := echo.New()
	handler := func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}
	return e, handler
}

func doRequest(e *echo.Echo, mw echo.MiddlewareFunc, handler echo.HandlerFunc, path string, user *model.User) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, nil)
	req.RemoteAddr = "192.168.1.100:12345"
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath(path)
	if user != nil {
		c.Set(string(UserContextKey), user)
	}
	wrapped := mw(handler)
	_ = wrapped(c)
	return rec
}

// --- RateLimiter Middleware tests ---

func TestMiddleware_NoLimitDefined(t *testing.T) {
	store := &mockLimitStore{}
	rl := NewRateLimiter(store, true, map[string]*EndpointLimit{})
	e, h := setupEcho(rl)

	rec := doRequest(e, rl.Middleware(), h, "/api/notes/show", nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, store.calls)
}

func TestMiddleware_WithinLimit(t *testing.T) {
	store := &mockLimitStore{
		results: []mockResult{
			{Info: LimitInfo{Remaining: 5, ResetMs: time.Now().Add(time.Hour).UnixMilli()}},
		},
	}
	limits := map[string]*EndpointLimit{
		"notes/create": {Duration: time.Hour, Max: 300},
	}
	rl := NewRateLimiter(store, true, limits)
	e, h := setupEcho(rl)

	rec := doRequest(e, rl.Middleware(), h, "/api/notes/create", &model.User{ID: "user1"})

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "5", rec.Header().Get("X-RateLimit-Remaining"))
	require.Len(t, store.calls, 1)
	assert.Equal(t, "user1:notes/create", store.calls[0].Key)
	assert.Equal(t, time.Hour, store.calls[0].Duration)
	assert.Equal(t, 300, store.calls[0].Max)
}

func TestMiddleware_ExceedsLimit(t *testing.T) {
	resetMs := time.Now().Add(30 * time.Second).UnixMilli()
	store := &mockLimitStore{
		results: []mockResult{
			{Info: LimitInfo{Remaining: 0, ResetMs: resetMs}},
		},
	}
	limits := map[string]*EndpointLimit{
		"notes/create": {Duration: time.Hour, Max: 300},
	}
	rl := NewRateLimiter(store, true, limits)
	e, h := setupEcho(rl)

	rec := doRequest(e, rl.Middleware(), h, "/api/notes/create", &model.User{ID: "user1"})

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	errObj := body["error"].(map[string]any)
	assert.Equal(t, "RATE_LIMIT_EXCEEDED", errObj["code"])
	assert.Equal(t, "d5826d14-3982-4d2e-8011-b9e9f02499ef", errObj["id"])
}

func TestMiddleware_AuthenticatedActor(t *testing.T) {
	store := &mockLimitStore{
		results: []mockResult{
			{Info: LimitInfo{Remaining: 10}},
		},
	}
	limits := map[string]*EndpointLimit{
		"notes/create": {Duration: time.Hour, Max: 300},
	}
	rl := NewRateLimiter(store, true, limits)
	e, h := setupEcho(rl)

	doRequest(e, rl.Middleware(), h, "/api/notes/create", &model.User{ID: "myuserid"})

	require.Len(t, store.calls, 1)
	assert.Equal(t, "myuserid:notes/create", store.calls[0].Key)
}

func TestMiddleware_UnauthenticatedIPActor(t *testing.T) {
	store := &mockLimitStore{
		results: []mockResult{
			{Info: LimitInfo{Remaining: 10}},
		},
	}
	limits := map[string]*EndpointLimit{
		"notes/create": {Duration: time.Hour, Max: 300},
	}
	rl := NewRateLimiter(store, true, limits)
	e, h := setupEcho(rl)

	rec := doRequest(e, rl.Middleware(), h, "/api/notes/create", nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, store.calls, 1)
	// キーがip-プレフィックスのIPハッシュを含む
	assert.Contains(t, store.calls[0].Key, "ip-")
	assert.Contains(t, store.calls[0].Key, ":notes/create")
}

func TestMiddleware_UnauthenticatedIPDisabled(t *testing.T) {
	store := &mockLimitStore{}
	limits := map[string]*EndpointLimit{
		"notes/create": {Duration: time.Hour, Max: 300},
	}
	rl := NewRateLimiter(store, false, limits)
	e, h := setupEcho(rl)

	rec := doRequest(e, rl.Middleware(), h, "/api/notes/create", nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, store.calls)
}

func TestMiddleware_MinIntervalCheckFirst(t *testing.T) {
	store := &mockLimitStore{
		results: []mockResult{
			{Info: LimitInfo{Remaining: 1}},  // minInterval: OK
			{Info: LimitInfo{Remaining: 50}}, // duration/max: OK
		},
	}
	limits := map[string]*EndpointLimit{
		"notes/delete": {Duration: time.Hour, Max: 300, MinInterval: time.Second},
	}
	rl := NewRateLimiter(store, true, limits)
	e, h := setupEcho(rl)

	rec := doRequest(e, rl.Middleware(), h, "/api/notes/delete", &model.User{ID: "u1"})

	assert.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, store.calls, 2)
	// 1回目: minIntervalチェック
	assert.Equal(t, "u1:notes/delete:min", store.calls[0].Key)
	assert.Equal(t, time.Second, store.calls[0].Duration)
	assert.Equal(t, 1, store.calls[0].Max)
	// 2回目: duration/maxチェック
	assert.Equal(t, "u1:notes/delete", store.calls[1].Key)
	assert.Equal(t, time.Hour, store.calls[1].Duration)
	assert.Equal(t, 300, store.calls[1].Max)
}

func TestMiddleware_MinIntervalBlock(t *testing.T) {
	resetMs := time.Now().Add(500 * time.Millisecond).UnixMilli()
	store := &mockLimitStore{
		results: []mockResult{
			{Info: LimitInfo{Remaining: 0, ResetMs: resetMs}}, // minInterval: blocked
		},
	}
	limits := map[string]*EndpointLimit{
		"notes/delete": {Duration: time.Hour, Max: 300, MinInterval: time.Second},
	}
	rl := NewRateLimiter(store, true, limits)
	e, h := setupEcho(rl)

	rec := doRequest(e, rl.Middleware(), h, "/api/notes/delete", &model.User{ID: "u1"})

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	// minIntervalでブロックされたらduration/maxチェックは呼ばれない
	require.Len(t, store.calls, 1)
}

func TestMiddleware_RedisError_FailOpen(t *testing.T) {
	store := &mockLimitStore{
		results: []mockResult{
			{Err: assert.AnError},
		},
	}
	limits := map[string]*EndpointLimit{
		"notes/create": {Duration: time.Hour, Max: 300},
	}
	rl := NewRateLimiter(store, true, limits)
	e, h := setupEcho(rl)

	rec := doRequest(e, rl.Middleware(), h, "/api/notes/create", &model.User{ID: "u1"})

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestMiddleware_RedisError_MinInterval_FailOpen(t *testing.T) {
	store := &mockLimitStore{
		results: []mockResult{
			{Err: assert.AnError},
		},
	}
	limits := map[string]*EndpointLimit{
		"notes/delete": {Duration: time.Hour, Max: 300, MinInterval: time.Second},
	}
	rl := NewRateLimiter(store, true, limits)
	e, h := setupEcho(rl)

	rec := doRequest(e, rl.Middleware(), h, "/api/notes/delete", &model.User{ID: "u1"})

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestMiddleware_RetryAfterHeader(t *testing.T) {
	// 30秒後にリセット
	resetMs := time.Now().Add(30 * time.Second).UnixMilli()
	store := &mockLimitStore{
		results: []mockResult{
			{Info: LimitInfo{Remaining: 0, ResetMs: resetMs}},
		},
	}
	limits := map[string]*EndpointLimit{
		"notes/create": {Duration: time.Hour, Max: 300},
	}
	rl := NewRateLimiter(store, true, limits)
	e, h := setupEcho(rl)

	rec := doRequest(e, rl.Middleware(), h, "/api/notes/create", &model.User{ID: "u1"})

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	retryAfter := rec.Header().Get("Retry-After")
	assert.NotEmpty(t, retryAfter)
	// 30±2秒の範囲であること
	var secs int
	_, _ = fmt.Sscanf(retryAfter, "%d", &secs)
	assert.InDelta(t, 30, secs, 2)
}

func TestMiddleware_NonAPIPath(t *testing.T) {
	store := &mockLimitStore{}
	limits := map[string]*EndpointLimit{
		"notes/create": {Duration: time.Hour, Max: 300},
	}
	rl := NewRateLimiter(store, true, limits)
	e, h := setupEcho(rl)

	// /api/のないパスはエンドポイント名が一致しないのでスルー
	rec := doRequest(e, rl.Middleware(), h, "/healthz", nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, store.calls)
}

// --- ipHash tests ---

func TestIpHash_IPv4(t *testing.T) {
	h := ipHash("192.168.1.1")
	assert.True(t, len(h) > 3)
	assert.True(t, h[:3] == "ip-")
}

func TestIpHash_IPv4_DifferentAddresses(t *testing.T) {
	h1 := ipHash("192.168.1.1")
	h2 := ipHash("10.0.0.1")
	assert.NotEqual(t, h1, h2)
}

func TestIpHash_IPv6(t *testing.T) {
	h := ipHash("2001:db8::1")
	assert.True(t, len(h) > 3)
	assert.True(t, h[:3] == "ip-")
}

func TestIpHash_IPv6_SameSubnet(t *testing.T) {
	// 同一/64サブネットのIPは同じハッシュ
	h1 := ipHash("2001:db8:1234:5678::1")
	h2 := ipHash("2001:db8:1234:5678::ffff")
	assert.Equal(t, h1, h2)
}

func TestIpHash_IPv6_DifferentSubnet(t *testing.T) {
	h1 := ipHash("2001:db8:1234:5678::1")
	h2 := ipHash("2001:db8:1234:9999::1")
	assert.NotEqual(t, h1, h2)
}

func TestIpHash_Invalid(t *testing.T) {
	h := ipHash("not-an-ip")
	assert.True(t, len(h) > 3)
	assert.True(t, h[:3] == "ip-")
}

func TestIpHash_Deterministic(t *testing.T) {
	h1 := ipHash("192.168.1.1")
	h2 := ipHash("192.168.1.1")
	assert.Equal(t, h1, h2)
}

// --- parseIntFromString ---

func TestParseIntFromString(t *testing.T) {
	assert.Equal(t, int64(12345), parseIntFromString("12345"))
	assert.Equal(t, int64(0), parseIntFromString(""))
	assert.Equal(t, int64(0), parseIntFromString("abc"))
}

// --- DefaultEndpointLimits ---

func TestDefaultEndpointLimits_NotEmpty(t *testing.T) {
	assert.Greater(t, len(DefaultEndpointLimits), 50)
}

func TestDefaultEndpointLimits_KnownEndpoints(t *testing.T) {
	cases := []struct {
		endpoint string
		max      int
	}{
		{"notes/create", 300},
		{"following/create", 100},
		{"blocking/create", 20},
		{"drive/files/create", 120},
		{"channels/create", 10},
		{"ap/show", 30},
	}
	for _, tc := range cases {
		t.Run(tc.endpoint, func(t *testing.T) {
			limit, ok := DefaultEndpointLimits[tc.endpoint]
			require.True(t, ok, "endpoint %s not found in limits", tc.endpoint)
			assert.Equal(t, tc.max, limit.Max)
		})
	}
}

func TestDefaultEndpointLimits_MinIntervalEndpoints(t *testing.T) {
	cases := []struct {
		endpoint    string
		minInterval time.Duration
	}{
		{"notes/delete", time.Second},
		{"notes/unrenote", time.Second},
		{"notes/reactions/delete", 3 * time.Second},
		{"bubble-game/register", 30 * time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.endpoint, func(t *testing.T) {
			limit, ok := DefaultEndpointLimits[tc.endpoint]
			require.True(t, ok)
			assert.Equal(t, tc.minInterval, limit.MinInterval)
		})
	}
}

func TestMiddleware_RetryAfterNegativeClampsToZero(t *testing.T) {
	// リセット時刻が過去の場合、Retry-Afterは0にクランプ
	resetMs := time.Now().Add(-10 * time.Second).UnixMilli()
	store := &mockLimitStore{
		results: []mockResult{
			{Info: LimitInfo{Remaining: 0, ResetMs: resetMs}},
		},
	}
	limits := map[string]*EndpointLimit{
		"notes/create": {Duration: time.Hour, Max: 300},
	}
	rl := NewRateLimiter(store, true, limits)
	e, h := setupEcho(rl)

	rec := doRequest(e, rl.Middleware(), h, "/api/notes/create", &model.User{ID: "u1"})

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Equal(t, "0", rec.Header().Get("Retry-After"))
}

// --- Redis integration test ---

func TestRedisRateLimitStore_Integration(t *testing.T) {
	ctx := context.Background()
	tr, err := testutil.SetupRedis(ctx)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer tr.Teardown(ctx)
	tr.FlushAll(ctx)

	store := NewRedisRateLimitStore(tr.Client)

	// 5リクエスト/10秒のリミット
	const maxReqs = 5
	dur := 10 * time.Second

	// ZCARDはZADD前に実行されるので、remaining=max-count(before add)。
	// maxReqs回は通る（remaining: max, max-1, ..., 1）
	for i := 0; i < maxReqs; i++ {
		info, err := store.Check(ctx, "test-user:test-ep", dur, maxReqs)
		require.NoError(t, err)
		assert.Equal(t, maxReqs-i, info.Remaining, "iteration %d", i)
	}

	// maxReqs+1回目はremaining=0（ブロック対象）
	info, err := store.Check(ctx, "test-user:test-ep", dur, maxReqs)
	require.NoError(t, err)
	assert.Equal(t, 0, info.Remaining)
	assert.Greater(t, info.ResetMs, time.Now().UnixMilli())
}

func TestRedisRateLimitStore_SeparateKeys(t *testing.T) {
	ctx := context.Background()
	tr, err := testutil.SetupRedis(ctx)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer tr.Teardown(ctx)
	tr.FlushAll(ctx)

	store := NewRedisRateLimitStore(tr.Client)
	dur := 10 * time.Second

	// 異なるキーは独立してカウント
	// ZCARDはZADD前なので1回目のremaining=max(=1)
	info1, err := store.Check(ctx, "user1:ep", dur, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, info1.Remaining) // max=1で1回目、ZCARD=0 → remaining=1

	info2, err := store.Check(ctx, "user2:ep", dur, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, info2.Remaining) // 別キーなのでこちらも1回目

	// 2回目はremaining=0
	info3, err := store.Check(ctx, "user1:ep", dur, 1)
	require.NoError(t, err)
	assert.Equal(t, 0, info3.Remaining)
}

func TestNewRedisRateLimiter_Integration(t *testing.T) {
	ctx := context.Background()
	tr, err := testutil.SetupRedis(ctx)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer tr.Teardown(ctx)
	tr.FlushAll(ctx)

	limits := map[string]*EndpointLimit{
		"notes/create": {Duration: time.Hour, Max: 2},
	}
	rl := NewRedisRateLimiter(tr.Client, true, limits)

	e := echo.New()
	handler := func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}
	mw := rl.Middleware()

	// 2回は通る
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/notes/create", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPath("/api/notes/create")
		c.Set(string(UserContextKey), &model.User{ID: "testuser"})
		_ = mw(handler)(c)
		assert.Equal(t, http.StatusOK, rec.Code, "request %d", i)
	}

	// 3回目は429
	req := httptest.NewRequest(http.MethodPost, "/api/notes/create", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/notes/create")
	c.Set(string(UserContextKey), &model.User{ID: "testuser"})
	_ = mw(handler)(c)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
}
