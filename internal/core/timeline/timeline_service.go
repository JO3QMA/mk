package timeline

import (
	"context"
	"errors"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

// MaxTimelineLength caps the number of IDs kept in each Redis timeline list.
// Misskey本家のデフォルトと同じ200。
const MaxTimelineLength = 200

// Errors returned by Service.
var (
	// ErrUnauthenticated is returned by Home/Hybrid timelines when no user is provided.
	ErrUnauthenticated = errors.New("user is required for this timeline")
)

// NoteSource is the minimum interface required by Service to resolve note IDs
// and to fall back to a database scan when Redis is empty.
type NoteSource interface {
	FindManyByIDsWithUser(ids []string) ([]*model.Note, error)
}

// Service exposes the four timeline endpoints (home/local/global/hybrid).
// Reads always go through Redis first; on a miss it falls back to a direct
// repository query.
type Service struct {
	fanout        *FanoutTimelineService
	noteRepo      repository.NoteRepository
	followingRepo repository.FollowingRepository
}

// NewService creates a new timeline Service.
func NewService(fanout *FanoutTimelineService, noteRepo repository.NoteRepository, followingRepo repository.FollowingRepository) *Service {
	return &Service{fanout: fanout, noteRepo: noteRepo, followingRepo: followingRepo}
}

// HomeTimeline returns the timeline for a logged-in user. The home timeline
// shows notes by users they follow plus their own notes.
func (s *Service) HomeTimeline(ctx context.Context, viewer *model.User, untilID, sinceID string, limit int, filter TimelineFilter) ([]*model.Note, error) {
	if viewer == nil {
		return nil, ErrUnauthenticated
	}
	if limit <= 0 {
		limit = 20
	}
	ids, err := s.fanout.Get(ctx, HomeTimelineName(viewer.ID), untilID, sinceID, limit)
	if err != nil {
		return nil, err
	}
	if len(ids) > 0 {
		notes, err := s.resolve(ids)
		if err != nil {
			return nil, err
		}
		notes = ApplyFilter(notes, viewer.ID, filter)
		if !filter.AllowPartial && len(notes) < limit {
			return s.noteRepo.ListHomeTimeline(viewer.ID, limit, sinceID, untilID, toDBFilter(filter, viewer.ID))
		}
		return notes, nil
	}
	return s.noteRepo.ListHomeTimeline(viewer.ID, limit, sinceID, untilID, toDBFilter(filter, viewer.ID))
}

// LocalTimeline returns notes posted by local users with public/home visibility.
func (s *Service) LocalTimeline(ctx context.Context, viewer *model.User, untilID, sinceID string, limit int, filter TimelineFilter) ([]*model.Note, error) {
	if limit <= 0 {
		limit = 20
	}
	viewerID := ""
	if viewer != nil {
		viewerID = viewer.ID
	}
	ids, err := s.fanout.Get(ctx, LocalTimeline, untilID, sinceID, limit)
	if err != nil {
		return nil, err
	}
	if len(ids) > 0 {
		notes, err := s.resolve(ids)
		if err != nil {
			return nil, err
		}
		notes = ApplyFilter(notes, viewerID, filter)
		if !filter.AllowPartial && len(notes) < limit {
			return s.noteRepo.ListLocalTimeline(limit, sinceID, untilID, toDBFilter(filter, viewerID))
		}
		return notes, nil
	}
	return s.noteRepo.ListLocalTimeline(limit, sinceID, untilID, toDBFilter(filter, viewerID))
}

// GlobalTimeline returns all public notes including federated remotes.
func (s *Service) GlobalTimeline(ctx context.Context, viewer *model.User, untilID, sinceID string, limit int, filter TimelineFilter) ([]*model.Note, error) {
	if limit <= 0 {
		limit = 20
	}
	viewerID := ""
	if viewer != nil {
		viewerID = viewer.ID
	}
	ids, err := s.fanout.Get(ctx, GlobalTimeline, untilID, sinceID, limit)
	if err != nil {
		return nil, err
	}
	if len(ids) > 0 {
		notes, err := s.resolve(ids)
		if err != nil {
			return nil, err
		}
		notes = ApplyFilter(notes, viewerID, filter)
		if !filter.AllowPartial && len(notes) < limit {
			return s.noteRepo.ListGlobalTimeline(limit, sinceID, untilID, toDBFilter(filter, viewerID))
		}
		return notes, nil
	}
	return s.noteRepo.ListGlobalTimeline(limit, sinceID, untilID, toDBFilter(filter, viewerID))
}

// HybridTimeline merges home and local timelines into a single feed.
func (s *Service) HybridTimeline(ctx context.Context, viewer *model.User, untilID, sinceID string, limit int, filter TimelineFilter) ([]*model.Note, error) {
	if viewer == nil {
		return nil, ErrUnauthenticated
	}
	if limit <= 0 {
		limit = 20
	}
	multi, err := s.fanout.GetMulti(ctx, []Name{HomeTimelineName(viewer.ID), LocalTimeline}, untilID, sinceID, limit)
	if err != nil {
		return nil, err
	}
	merged := mergeIDs(multi, limit)
	if len(merged) > 0 {
		notes, err := s.resolve(merged)
		if err != nil {
			return nil, err
		}
		notes = ApplyFilter(notes, viewer.ID, filter)
		if !filter.AllowPartial && len(notes) < limit {
			return s.noteRepo.ListHomeTimeline(viewer.ID, limit, sinceID, untilID, toDBFilter(filter, viewer.ID))
		}
		return notes, nil
	}
	return s.noteRepo.ListHomeTimeline(viewer.ID, limit, sinceID, untilID, toDBFilter(filter, viewer.ID))
}

// toDBFilter converts a TimelineFilter to a model.TimelineDBFilter.
func toDBFilter(f TimelineFilter, viewerID string) model.TimelineDBFilter {
	return model.TimelineDBFilter{
		WithFiles:             f.WithFiles,
		WithRenotes:           f.WithRenotes,
		WithReplies:           f.WithReplies,
		IncludeMyRenotes:      f.IncludeMyRenotes,
		IncludeRenotedMyNotes: f.IncludeRenotedMyNotes,
		IncludeLocalRenotes:   f.IncludeLocalRenotes,
		ViewerID:              viewerID,
		MutedChannelIDs:       f.MutedChannelIDs,
	}
}

// resolve fetches notes from the repository preserving id ordering.
func (s *Service) resolve(ids []string) ([]*model.Note, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	return s.noteRepo.FindManyByIDsWithUser(ids)
}

// mergeIDs flattens multiple ID slices, deduplicates, sorts id desc and caps.
func mergeIDs(slices [][]string, limit int) []string {
	seen := make(map[string]struct{})
	var all []string
	for _, s := range slices {
		for _, id := range s {
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
			all = append(all, id)
		}
	}
	// id降順
	for i := 0; i < len(all); i++ {
		for j := i + 1; j < len(all); j++ {
			if all[i] < all[j] {
				all[i], all[j] = all[j], all[i]
			}
		}
	}
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all
}
