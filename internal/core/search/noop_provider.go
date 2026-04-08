package search

import "github.com/shiroha-a/mk/internal/model"

// NoopProvider is the default Provider used when search is disabled in the
// configuration.  All operations are no-ops; SearchNote always returns an
// empty slice.
type NoopProvider struct{}

// NewNoopProvider returns a Provider whose operations all succeed without
// touching any external system.
func NewNoopProvider() *NoopProvider { return &NoopProvider{} }

// IndexNote is a no-op.
func (p *NoopProvider) IndexNote(_ *model.Note) error { return nil }

// UnindexNote is a no-op.
func (p *NoopProvider) UnindexNote(_ *model.Note) error { return nil }

// SearchNote always returns an empty slice.
func (p *NoopProvider) SearchNote(_ *model.User, _ string, _ SearchOpts, _ Pagination) ([]*model.Note, error) {
	return nil, nil
}
