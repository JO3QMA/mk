package activitypub

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddContext_Person(t *testing.T) {
	p := &Person{}
	AddContext(p)
	ctx, ok := p.Context.([]any)
	assert.True(t, ok)
	assert.Contains(t, ctx, ContextURL)
	assert.Contains(t, ctx, SecurityContextURL)
}

func TestAddContext_Variants(t *testing.T) {
	cases := []any{
		&Note{},
		&Create{},
		&Follow{},
		&Accept{},
		&Reject{},
		&Undo{},
		&Delete{},
		&Update{},
		&Like{},
		&Announce{},
	}
	for _, c := range cases {
		AddContext(c)
		// 全型で fullContext (AS + Security + MisskeyContext) が設定される
		var ctx any
		switch v := c.(type) {
		case *Note:
			ctx = v.Context
		case *Create:
			ctx = v.Context
		case *Follow:
			ctx = v.Context
		case *Accept:
			ctx = v.Context
		case *Reject:
			ctx = v.Context
		case *Undo:
			ctx = v.Context
		case *Delete:
			ctx = v.Context
		case *Update:
			ctx = v.Context
		case *Like:
			ctx = v.Context
		case *Announce:
			ctx = v.Context
		}
		arr, ok := ctx.([]any)
		assert.True(t, ok)
		assert.Contains(t, arr, ContextURL)
		assert.Contains(t, arr, SecurityContextURL)
	}
}

func TestAddContext_IndependentSlices(t *testing.T) {
	// 異なるオブジェクトが独立したcontextスライスを持つこと
	p := &Person{}
	n := &Note{}
	AddContext(p)
	AddContext(n)
	pCtx := p.Context.([]any)
	nCtx := n.Context.([]any)
	// appendしても互いに影響しない
	pCtx = append(pCtx, "extra")
	assert.Len(t, nCtx, 3)
}

func TestAddContext_NoOpForUnknown(t *testing.T) {
	// 引数の型が switch にない場合はno-op
	type other struct{}
	AddContext(&other{})
}

func TestPersonMarshalsCleanly(t *testing.T) {
	p := &Person{
		Object: Object{ID: "https://example.com/users/u1", Type: "Person"},
	}
	AddContext(p)
	b, err := json.Marshal(p)
	assert.NoError(t, err)
	assert.Contains(t, string(b), "Person")
	assert.Contains(t, string(b), ContextURL)
}

func TestNewMention(t *testing.T) {
	m := NewMention("https://example.com/users/u1", "@u1")
	assert.Equal(t, "Mention", m.Type)
	assert.Equal(t, "https://example.com/users/u1", m.Href)
	assert.Equal(t, "@u1", m.Name)

	// name が空でも factory は Type を必ず埋める
	m2 := NewMention("https://example.com/users/u2", "")
	assert.Equal(t, "Mention", m2.Type)
	assert.Empty(t, m2.Name)

	// JSON 出力時も type フィールドが出ていること
	b, err := json.Marshal(m)
	assert.NoError(t, err)
	assert.Contains(t, string(b), `"type":"Mention"`)
}

func TestIsValidActorType(t *testing.T) {
	cases := []struct {
		typ  string
		want bool
	}{
		{"Person", true},
		{"Service", true},
		{"Group", true},
		{"Organization", true},
		{"Application", true},
		{"Note", false},
		{"Tombstone", false},
		{"", false},
		{"person", false}, // case sensitive (TS と同じ)
	}
	for _, c := range cases {
		t.Run(c.typ, func(t *testing.T) {
			assert.Equal(t, c.want, IsValidActorType(c.typ))
		})
	}
}

func TestIsBotActorType(t *testing.T) {
	cases := []struct {
		typ  string
		want bool
	}{
		{"Service", true},
		{"Application", true},
		{"Person", false},
		{"Group", false},
		{"Organization", false},
		{"", false},
	}
	for _, c := range cases {
		t.Run(c.typ, func(t *testing.T) {
			assert.Equal(t, c.want, IsBotActorType(c.typ))
		})
	}
}
