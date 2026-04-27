package processors

import (
	"context"
	"testing"

	"github.com/shiroha-a/mk/internal/core/reaction"
	"github.com/shiroha-a/mk/internal/queue/driver"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestReactionFlushProcessor_Handle(t *testing.T) {
	noteRepo := testutil.NewMockNoteRepository()
	writer := reaction.NewDirectWriter(noteRepo)
	p := NewReactionFlushProcessor(writer)
	require.NoError(t, p.Handle(context.Background(), driver.RawTask{TypeName: "test"}))
}
