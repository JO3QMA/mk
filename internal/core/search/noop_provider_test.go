package search

import (
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoopProvider_AllOpsAreNoOps(t *testing.T) {
	p := NewNoopProvider()
	require.NoError(t, p.IndexNote(&model.Note{ID: "x"}))
	require.NoError(t, p.UnindexNote(&model.Note{ID: "x"}))
	out, err := p.SearchNote(nil, "anything", SearchOpts{UserID: "u1"}, Pagination{Limit: 10})
	require.NoError(t, err)
	assert.Empty(t, out)
}
