package processors

import (
	"context"

	"github.com/shiroha-a/mk/internal/core/reaction"
	"github.com/shiroha-a/mk/internal/queue/driver"
)

// ReactionFlushProcessor handles the periodic reaction buffer flush task.
type ReactionFlushProcessor struct {
	writer reaction.ReactionCountWriter
}

// NewReactionFlushProcessor creates the processor.
func NewReactionFlushProcessor(writer reaction.ReactionCountWriter) *ReactionFlushProcessor {
	return &ReactionFlushProcessor{writer: writer}
}

// Handle flushes all buffered reaction counts to the database.
func (p *ReactionFlushProcessor) Handle(ctx context.Context, _ driver.Task) error {
	return p.writer.Flush(ctx)
}
