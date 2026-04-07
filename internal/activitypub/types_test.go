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
		// 全て single string context (Person 以外)
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
		assert.Equal(t, ContextURL, ctx)
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
