package processors

import (
	"context"
	"errors"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/shiroha-a/mk/internal/core/transfer"
	"github.com/shiroha-a/mk/internal/queue"
)

// ExportProcessor handles `export` tasks by delegating to transfer.Exporter.
type ExportProcessor struct {
	exporter *transfer.Exporter
}

// NewExportProcessor constructs an ExportProcessor.
func NewExportProcessor(exporter *transfer.Exporter) *ExportProcessor {
	return &ExportProcessor{exporter: exporter}
}

// Handle dispatches a single export task.
func (p *ExportProcessor) Handle(ctx context.Context, t *asynq.Task) error {
	if p.exporter == nil {
		return fmt.Errorf("export processor not configured: %w", asynq.SkipRetry)
	}
	payload, err := queue.DecodeExportPayload(t.Payload())
	if err != nil {
		return fmt.Errorf("decode export payload: %w: %w", err, asynq.SkipRetry)
	}
	if payload.UserID == "" || payload.Type == "" {
		return fmt.Errorf("export payload missing fields: %w", asynq.SkipRetry)
	}
	if _, err := p.exporter.Export(ctx, payload.UserID, payload.Type); err != nil {
		// 未対応typeはリトライしても通らないので SkipRetry。
		if errors.Is(err, transfer.ErrUnsupportedType) {
			return fmt.Errorf("%w: %w", err, asynq.SkipRetry)
		}
		return err
	}
	return nil
}

// ImportProcessor handles `import` tasks by delegating to transfer.Importer.
type ImportProcessor struct {
	importer *transfer.Importer
}

// NewImportProcessor constructs an ImportProcessor.
func NewImportProcessor(importer *transfer.Importer) *ImportProcessor {
	return &ImportProcessor{importer: importer}
}

// Handle dispatches a single import task.
func (p *ImportProcessor) Handle(ctx context.Context, t *asynq.Task) error {
	if p.importer == nil {
		return fmt.Errorf("import processor not configured: %w", asynq.SkipRetry)
	}
	payload, err := queue.DecodeImportPayload(t.Payload())
	if err != nil {
		return fmt.Errorf("decode import payload: %w: %w", err, asynq.SkipRetry)
	}
	if payload.UserID == "" || payload.Type == "" || payload.FileID == "" {
		return fmt.Errorf("import payload missing fields: %w", asynq.SkipRetry)
	}
	if _, err := p.importer.Import(ctx, payload.UserID, payload.Type, payload.FileID); err != nil {
		if errors.Is(err, transfer.ErrUnsupportedType) {
			return fmt.Errorf("%w: %w", err, asynq.SkipRetry)
		}
		return err
	}
	return nil
}
