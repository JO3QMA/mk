package processors

import (
	"context"
	"testing"

	"github.com/shiroha-a/mk/internal/queue/driver"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanRemoteNotes_Disabled(t *testing.T) {
	noteRepo := testutil.NewMockNoteRepository()
	p := NewCleanRemoteNotesProcessor(noteRepo, CleanRemoteNotesConfig{Enabled: false})
	err := p.Handle(context.Background(), driver.RawTask{TypeName: "test"})
	require.NoError(t, err)
}

func TestCleanRemoteNotes_Enabled(t *testing.T) {
	noteRepo := testutil.NewMockNoteRepository()
	p := NewCleanRemoteNotesProcessor(noteRepo, CleanRemoteNotesConfig{
		Enabled:              true,
		ExpiryDays:           90,
		MaxProcessingMinutes: 1,
	})
	err := p.Handle(context.Background(), driver.RawTask{TypeName: "test"})
	require.NoError(t, err)
}

func TestCleanRemoteNotes_DefaultValues(t *testing.T) {
	noteRepo := testutil.NewMockNoteRepository()
	p := NewCleanRemoteNotesProcessor(noteRepo, CleanRemoteNotesConfig{
		Enabled:              true,
		ExpiryDays:           0,
		MaxProcessingMinutes: 0,
	})
	err := p.Handle(context.Background(), driver.RawTask{TypeName: "test"})
	require.NoError(t, err)
}

func TestCleanRemoteNotes_CancelledContext(t *testing.T) {
	noteRepo := testutil.NewMockNoteRepository()
	p := NewCleanRemoteNotesProcessor(noteRepo, CleanRemoteNotesConfig{
		Enabled:              true,
		ExpiryDays:           90,
		MaxProcessingMinutes: 60,
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := p.Handle(ctx, driver.RawTask{TypeName: "test"})
	assert.NoError(t, err)
}
