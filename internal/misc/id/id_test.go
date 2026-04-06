package id

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGenerator(t *testing.T) {
	methods := []string{"aid", "aidx", "meid", "objectid", "ulid"}
	for _, m := range methods {
		t.Run(m, func(t *testing.T) {
			g, err := NewGenerator(m)
			require.NoError(t, err)
			assert.NotNil(t, g)
		})
	}

	t.Run("unknown method", func(t *testing.T) {
		_, err := NewGenerator("invalid")
		assert.Error(t, err)
	})
}

func TestAID_GenerateAndParse(t *testing.T) {
	g, _ := NewGenerator("aid")
	now := time.Now()

	id := g.Generate(now)
	assert.Len(t, id, 10)

	parsed, err := g.ParseTime(id)
	require.NoError(t, err)
	assert.WithinDuration(t, now, parsed, time.Millisecond)
}

func TestAID_Uniqueness(t *testing.T) {
	g, _ := NewGenerator("aid")
	now := time.Now()
	seen := make(map[string]bool)

	for i := 0; i < 100; i++ {
		id := g.Generate(now)
		assert.False(t, seen[id], "duplicate ID: %s", id)
		seen[id] = true
	}
}

func TestAIDX_GenerateAndParse(t *testing.T) {
	g, _ := NewGenerator("aidx")
	now := time.Now()

	id := g.Generate(now)
	assert.Len(t, id, 16)

	parsed, err := g.ParseTime(id)
	require.NoError(t, err)
	assert.WithinDuration(t, now, parsed, time.Millisecond)
}

func TestAIDX_Uniqueness(t *testing.T) {
	g, _ := NewGenerator("aidx")
	now := time.Now()
	seen := make(map[string]bool)

	for i := 0; i < 100; i++ {
		id := g.Generate(now)
		assert.False(t, seen[id], "duplicate ID: %s", id)
		seen[id] = true
	}
}

func TestMEID_GenerateAndParse(t *testing.T) {
	g, _ := NewGenerator("meid")
	now := time.Now()

	id := g.Generate(now)
	assert.Len(t, id, 24)

	parsed, err := g.ParseTime(id)
	require.NoError(t, err)
	assert.WithinDuration(t, now, parsed, time.Millisecond)
}

func TestObjectID_GenerateAndParse(t *testing.T) {
	g, _ := NewGenerator("objectid")
	now := time.Now()

	id := g.Generate(now)
	assert.Len(t, id, 24)

	parsed, err := g.ParseTime(id)
	require.NoError(t, err)
	// ObjectID uses seconds precision
	assert.WithinDuration(t, now, parsed, time.Second)
}

func TestULID_GenerateAndParse(t *testing.T) {
	g, _ := NewGenerator("ulid")
	now := time.Now()

	id := g.Generate(now)
	assert.Len(t, id, 26)

	parsed, err := g.ParseTime(id)
	require.NoError(t, err)
	assert.WithinDuration(t, now, parsed, time.Millisecond)
}

func TestULID_Monotonic(t *testing.T) {
	g, _ := NewGenerator("ulid")
	now := time.Now()

	id1 := g.Generate(now)
	id2 := g.Generate(now)
	// 同一時刻でもID値は単調増加する
	assert.True(t, id2 > id1, "ULID should be monotonically increasing")
}

func TestParseTime_InvalidInput(t *testing.T) {
	methods := []string{"aid", "aidx", "meid", "objectid", "ulid"}
	for _, m := range methods {
		t.Run(m, func(t *testing.T) {
			g, _ := NewGenerator(m)
			_, err := g.ParseTime("x")
			assert.Error(t, err)
		})
	}
}

func TestGenerators_Ordering(t *testing.T) {
	// ObjectIDは秒精度のため、このテストでは除外
	methods := []string{"aid", "aidx", "meid", "ulid"}
	for _, m := range methods {
		t.Run(m, func(t *testing.T) {
			g, _ := NewGenerator(m)
			t1 := time.Now()
			id1 := g.Generate(t1)

			time.Sleep(2 * time.Millisecond)

			t2 := time.Now()
			id2 := g.Generate(t2)

			// 後に生成されたIDは文字列比較で大きい
			assert.True(t, id2 > id1, "later ID should be lexicographically greater")
		})
	}
}

func TestObjectID_Ordering(t *testing.T) {
	g, _ := NewGenerator("objectid")
	t1 := time.Now()
	id1 := g.Generate(t1)

	// ObjectIDは秒精度なので1秒以上空ける
	t2 := t1.Add(2 * time.Second)
	id2 := g.Generate(t2)

	assert.True(t, id2 > id1, "later ObjectID should be lexicographically greater")
}
