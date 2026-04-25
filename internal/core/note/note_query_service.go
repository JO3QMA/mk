package note

import (
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

// QueryService provides read-only note queries used by notes/* endpoints.
type QueryService struct {
	noteRepo       repository.NoteRepository
	followingRepo  repository.FollowingRepository
	favoriteRepo   repository.NoteFavoriteRepository
	threadMuteRepo repository.NoteThreadMutingRepository
}

// NewQueryService creates a new QueryService.
// followingRepoはfollowers可視性のチェックに使われる。nilを許容する。
func NewQueryService(noteRepo repository.NoteRepository, followingRepo repository.FollowingRepository) *QueryService {
	return &QueryService{noteRepo: noteRepo, followingRepo: followingRepo}
}

// SetFavoriteRepo enables isFavorited reporting on State(). nil でも State
// は動くが isFavorited は常に false になる。
func (s *QueryService) SetFavoriteRepo(r repository.NoteFavoriteRepository) {
	s.favoriteRepo = r
}

// SetThreadMutingRepo enables isMutedThread reporting on State(). nil でも
// State は動くが isMutedThread は常に false になる。
func (s *QueryService) SetThreadMutingRepo(r repository.NoteThreadMutingRepository) {
	s.threadMuteRepo = r
}

// Show returns the requested note if it exists and the viewer can see it.
// viewerはnil可 (未認証ユーザー)。
func (s *QueryService) Show(viewer *model.User, noteID string) (*model.Note, error) {
	n, err := s.noteRepo.FindByIDWithRelations(noteID)
	if err != nil {
		return nil, ErrNoteNotFound
	}
	if !CanSeeNote(viewer, n, s.followingRepo) {
		return nil, ErrNoteNotFound
	}
	return n, nil
}

// ListRenotes returns the renotes of the given noteID after filtering for visibility.
// 元のノートが閲覧できない場合はErrNoteNotFoundを返す。
func (s *QueryService) ListRenotes(viewer *model.User, noteID, untilID, sinceID string, limit int) ([]*model.Note, error) {
	if _, err := s.Show(viewer, noteID); err != nil {
		return nil, err
	}
	rows, err := s.noteRepo.ListRenotesOf(noteID, untilID, sinceID, limit)
	if err != nil {
		return nil, err
	}
	return s.filterVisible(viewer, rows), nil
}

// ListReplies returns replies to the given noteID after filtering for visibility.
func (s *QueryService) ListReplies(viewer *model.User, noteID, untilID, sinceID string, limit int) ([]*model.Note, error) {
	if _, err := s.Show(viewer, noteID); err != nil {
		return nil, err
	}
	rows, err := s.noteRepo.ListRepliesOf(noteID, untilID, sinceID, limit)
	if err != nil {
		return nil, err
	}
	return s.filterVisible(viewer, rows), nil
}

// ListChildren returns notes that reply to or quote the given noteID.
func (s *QueryService) ListChildren(viewer *model.User, noteID, untilID, sinceID string, limit int) ([]*model.Note, error) {
	if _, err := s.Show(viewer, noteID); err != nil {
		return nil, err
	}
	rows, err := s.noteRepo.ListChildrenOf(noteID, untilID, sinceID, limit)
	if err != nil {
		return nil, err
	}
	return s.filterVisible(viewer, rows), nil
}

// Conversation walks up the reply chain from the given noteID and returns
// up to `limit` ancestors, ordered from oldest to newest. The starting note
// itself is NOT included. Notes the viewer cannot see terminate the walk.
func (s *QueryService) Conversation(viewer *model.User, noteID string, limit int) ([]*model.Note, error) {
	start, err := s.Show(viewer, noteID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 10
	}

	var ancestors []*model.Note
	current := start
	// 親をたどる。深さ制限はlimit回。サイクル回避のため訪問済みも記録する。
	visited := map[string]struct{}{current.ID: {}}
	for len(ancestors) < limit {
		if current.ReplyID == nil {
			break
		}
		parentID := *current.ReplyID
		if _, dup := visited[parentID]; dup {
			break
		}
		parent, err := s.noteRepo.FindByIDWithRelations(parentID)
		if err != nil {
			break
		}
		if !CanSeeNote(viewer, parent, s.followingRepo) {
			break
		}
		visited[parentID] = struct{}{}
		ancestors = append(ancestors, parent)
		current = parent
	}
	// 古い順 (最深の親を先頭) に並び替える
	reverseNotes(ancestors)
	return ancestors, nil
}

// State returns the note's user-specific state flags.
// 匿名閲覧者 (viewer==nil) はすべて false。
//
// upstream Misskey の notes/state は isFavorited / isMutedThread のみを返す
// (isWatching は廃止) ため、こちらも同じ shape に揃える。
func (s *QueryService) State(viewer *model.User, noteID string) (*NoteState, error) {
	note, err := s.Show(viewer, noteID)
	if err != nil {
		return nil, err
	}
	state := &NoteState{}
	if viewer == nil {
		return state, nil
	}
	if s.favoriteRepo != nil {
		if ok, err := s.favoriteRepo.Exists(viewer.ID, note.ID); err == nil && ok {
			state.IsFavorited = true
		}
	}
	if s.threadMuteRepo != nil {
		// 本家 Misskey の threadId は note.threadId (ルート) を指す。reply 系の
		// ノートは自身の threadId を、ルートは自身の id を threadId として扱う。
		threadID := note.ID
		if note.ThreadID != nil && *note.ThreadID != "" {
			threadID = *note.ThreadID
		}
		if ok, err := s.threadMuteRepo.Exists(viewer.ID, threadID); err == nil && ok {
			state.IsMutedThread = true
		}
	}
	return state, nil
}

// NoteState represents notes/state response payload.
type NoteState struct {
	IsFavorited   bool `json:"isFavorited"`
	IsMutedThread bool `json:"isMutedThread"`
}

// FilterVisible drops notes the viewer cannot see. Public API for use by
// handlers that perform bulk lookups outside of QueryService (e.g. BulkShow).
func (s *QueryService) FilterVisible(viewer *model.User, rows []*model.Note) []*model.Note {
	return s.filterVisible(viewer, rows)
}

// filterVisible drops notes the viewer cannot see.
func (s *QueryService) filterVisible(viewer *model.User, rows []*model.Note) []*model.Note {
	out := make([]*model.Note, 0, len(rows))
	for _, n := range rows {
		if CanSeeNote(viewer, n, s.followingRepo) {
			out = append(out, n)
		}
	}
	return out
}

func reverseNotes(notes []*model.Note) {
	for i, j := 0, len(notes)-1; i < j; i, j = i+1, j-1 {
		notes[i], notes[j] = notes[j], notes[i]
	}
}
