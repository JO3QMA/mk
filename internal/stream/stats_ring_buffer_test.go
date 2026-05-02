package stream

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRingBuffer_AppendAndLog_NewestFirst(t *testing.T) {
	rb := newStatsRingBuffer(5)
	rb.Append(json.RawMessage(`"a"`))
	rb.Append(json.RawMessage(`"b"`))
	rb.Append(json.RawMessage(`"c"`))

	got := rb.Log(0)
	require.Len(t, got, 3)
	assert.Equal(t, `"c"`, string(got[0]), "newest first")
	assert.Equal(t, `"b"`, string(got[1]))
	assert.Equal(t, `"a"`, string(got[2]))
}

func TestRingBuffer_OverwritesOldest(t *testing.T) {
	rb := newStatsRingBuffer(3)
	for _, v := range []string{`"a"`, `"b"`, `"c"`, `"d"`, `"e"`} {
		rb.Append(json.RawMessage(v))
	}
	got := rb.Log(0)
	// cap=3 で a,b は消えて e,d,c が残る (newest first)
	require.Len(t, got, 3)
	assert.Equal(t, `"e"`, string(got[0]))
	assert.Equal(t, `"d"`, string(got[1]))
	assert.Equal(t, `"c"`, string(got[2]))
}

func TestRingBuffer_LogMaxLenClamping(t *testing.T) {
	rb := newStatsRingBuffer(10)
	for i := 0; i < 10; i++ {
		rb.Append(json.RawMessage(`{}`))
	}
	assert.Len(t, rb.Log(0), 10, "maxLen=0 → cap")
	assert.Len(t, rb.Log(-1), 10, "maxLen<0 → cap")
	assert.Len(t, rb.Log(100), 10, "maxLen>cap → cap")
	assert.Len(t, rb.Log(3), 3)
}

func TestRingBuffer_DefensiveCopy_OnAppend(t *testing.T) {
	rb := newStatsRingBuffer(2)
	mutable := json.RawMessage(`{"k":1}`)
	rb.Append(mutable)
	// caller が input を mutate しても内部に反映されない
	mutable[1] = 'x' // `{` → `xk":1}` に破壊的編集
	got := rb.Log(0)
	require.Len(t, got, 1)
	assert.Equal(t, `{"k":1}`, string(got[0]))
}

func TestRingBuffer_DefensiveCopy_OnLog(t *testing.T) {
	rb := newStatsRingBuffer(2)
	rb.Append(json.RawMessage(`"a"`))

	got1 := rb.Log(0)
	got1[0] = json.RawMessage(`"x"`) // caller mutation
	got2 := rb.Log(0)
	require.Len(t, got2, 1)
	assert.Equal(t, `"a"`, string(got2[0]), "internal buffer not aliased to caller's slice")
}

func TestRingBuffer_ConcurrentAppendLog(t *testing.T) {
	// race detector 有効環境 (-race) で並行 Append/Log が安全であることを
	// 確認する。何回か回してもデータが consistent。
	rb := newStatsRingBuffer(50)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				rb.Append(json.RawMessage(`{}`))
			}
		}()
	}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = rb.Log(0)
			}
		}()
	}
	wg.Wait()
	// 1000 append したので cap=50 で頭打ち
	assert.Len(t, rb.Log(0), 50)
}

// capacity<=0 で panic
func TestRingBuffer_PanicOnInvalidCap(t *testing.T) {
	assert.Panics(t, func() { newStatsRingBuffer(0) })
	assert.Panics(t, func() { newStatsRingBuffer(-1) })
}
