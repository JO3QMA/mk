package search

import (
	"github.com/shiroha-a/mk/internal/core/note"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

// SQLLikeProvider is the default backend that delegates to
// `noteRepo.SearchByFilter`. It performs a case-insensitive substring match
// over public/home notes and supports the same filter options as the
// Meilisearch backend (userId / channelId / host).
//
// Index / Unindex are no-ops because the canonical store is the database
// itself; nothing extra has to happen on note creation / deletion.
type SQLLikeProvider struct {
	noteRepo      repository.NoteRepository
	followingRepo repository.FollowingRepository
}

// NewSQLLikeProvider returns a Provider backed by SearchByFilter on the
// supplied note repository. followingRepo は visibility 後フィルタ
// (note.CanSeeNote) で followers 可視性のチェックに使う。nil 可。
func NewSQLLikeProvider(noteRepo repository.NoteRepository, followingRepo repository.FollowingRepository) *SQLLikeProvider {
	return &SQLLikeProvider{noteRepo: noteRepo, followingRepo: followingRepo}
}

// IndexNote is a no-op for the SQL backend (the database is the index).
func (p *SQLLikeProvider) IndexNote(_ *model.Note) error { return nil }

// UnindexNote is a no-op for the SQL backend.
func (p *SQLLikeProvider) UnindexNote(_ *model.Note) error { return nil }

// SearchNote runs an ILIKE-based search and post-filters the result for
// viewer visibility.
func (p *SQLLikeProvider) SearchNote(viewer *model.User, query string, opts SearchOpts, page Pagination) ([]*model.Note, error) {
	if query == "" {
		return nil, ErrEmptyQuery
	}
	limit := page.Limit
	if limit <= 0 {
		limit = 10
	}
	rows, err := p.noteRepo.SearchByFilter(model.NoteSearchFilter{
		Query:     query,
		UserID:    opts.UserID,
		ChannelID: opts.ChannelID,
		Host:      opts.Host,
		UntilID:   page.UntilID,
		SinceID:   page.SinceID,
		Limit:     limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*model.Note, 0, len(rows))
	for _, n := range rows {
		if note.CanSeeNote(viewer, n, p.followingRepo) {
			out = append(out, n)
		}
	}
	return out, nil
}
