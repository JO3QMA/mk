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

// issue #388: 自前実装では下位5bit truncationにより1/32の確率で単調性が
// 破れたflakyバグがあった。oklog/ulid の80bit monotonic entropyに置き換
// えた後は1000回連続で単調性が保たれることを検証する。
func TestULID_MonotonicStress(t *testing.T) {
	g, _ := NewGenerator("ulid")
	now := time.Now()
	prev := g.Generate(now)
	for i := 0; i < 1000; i++ {
		cur := g.Generate(now)
		require.Greater(t, cur, prev, "monotonic broken at iteration %d: prev=%s cur=%s", i, prev, cur)
		prev = cur
	}
}

// upstream Misskey #17310 (= 2026.5.0 fix) は自前 parser が一般 base32 を使って
// いて Crockford 専用の W/X/Y/Z (値 24-31) を誤って parse する bug を修正したが、
// mk-go は oklog/ulid/v2 に委譲しており仕様準拠の Crockford parser を持つため
// 同 bug は構造的に発生しない。regression guard として W/X/Y/Z を含む ULID を
// 直接 parse して timestamp が抽出できることを確認する
// (= triage #1005 / upstream #17310 close)。
func TestULID_CrockfordWXYZ(t *testing.T) {
	g, _ := NewGenerator("ulid")
	tests := []struct {
		name string
		id   string
	}{
		// W/X/Y/Z を randomness 部 (= 後半 16 char) に含む 26 文字 ULID。
		// timestamp 部は適当な値で良い (Crockford 0-9A-HJKMNP-TV-Z)。
		{"contains_W", "01HZWWWWWWWWWWWWWWWWWWWWWW"},
		{"contains_X", "01HZXXXXXXXXXXXXXXXXXXXXXX"},
		{"contains_Y", "01HZYYYYYYYYYYYYYYYYYYYYYY"},
		{"contains_Z", "01HZZZZZZZZZZZZZZZZZZZZZZZ"},
		// 全 boundary 値が混在するケース
		{"mixed_high_chars", "01HGWXYZWXYZWXYZWXYZWXYZWX"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := g.ParseTime(tc.id)
			require.NoError(t, err, "Crockford W/X/Y/Z must parse without error (upstream #17310 regression)")
		})
	}
}

func TestParseTime_InvalidInput(t *testing.T) {
	methods := []string{"aid", "aidx", "meid", "objectid", "ulid"}
	for _, m := range methods {
		t.Run(m+"_too_short", func(t *testing.T) {
			g, _ := NewGenerator(m)
			_, err := g.ParseTime("x")
			assert.Error(t, err)
		})
	}
}

func TestParseTime_InvalidChars(t *testing.T) {
	tests := []struct {
		method string
		id     string
	}{
		// 長さは足りるがParseIntが失敗する文字を含む
		{"aid", "!!!!!!!!00"},        // base36に!は無効
		{"aidx", "!!!!!!!!00000000"}, // base36に!は無効
		{"meid", "ZZZZZZZZZZZZ000000000000"},
		{"objectid", "ZZZZZZZZ0000000000000000"},
		{"ulid", "&&&&&&&&&&0000000000000000"},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			g, _ := NewGenerator(tt.method)
			_, err := g.ParseTime(tt.id)
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
