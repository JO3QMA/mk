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
