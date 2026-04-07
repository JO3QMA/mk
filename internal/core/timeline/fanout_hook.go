package timeline

import (
	"context"
	"log/slog"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

// FanoutHook implements note.TimelineFanoutHook by pushing newly-created notes
// onto the appropriate Redis timelines (home/local/global/user).
type FanoutHook struct {
	fanout        *FanoutTimelineService
	followingRepo repository.FollowingRepository
}

// NewFanoutHook constructs a FanoutHook.
func NewFanoutHook(fanout *FanoutTimelineService, followingRepo repository.FollowingRepository) *FanoutHook {
	return &FanoutHook{fanout: fanout, followingRepo: followingRepo}
}

// OnNoteCreated delivers the given note to user/home/local/global timelines.
// 配信失敗はログに記録するだけで上位に伝搬しない (ベストエフォート)。
func (h *FanoutHook) OnNoteCreated(n *model.Note, author *model.User) {
	if n == nil || author == nil {
		return
	}
	ctx := context.Background()

	// 1. ユーザータイムライン (本人の投稿一覧)
	h.push(ctx, UserTimelineName(author.ID), n.ID)

	// 2. ホームタイムライン: 投稿者本人 + フォロワー全員
	//    follower一覧の取得は1ページずつ繰り返し読みだす
	h.push(ctx, HomeTimelineName(author.ID), n.ID)
	if h.followingRepo != nil && shouldFanoutToFollowers(n) {
		h.fanoutToFollowers(ctx, author.ID, n.ID)
	}

	// 3. ローカルタイムライン: ローカル投稿でvisibility=public/homeのみ
	if author.Host == nil && (n.Visibility == model.NoteVisibilityPublic || n.Visibility == model.NoteVisibilityHome) {
		h.push(ctx, LocalTimeline, n.ID)
	}

	// 4. グローバルタイムライン: visibility=publicのみ
	if n.Visibility == model.NoteVisibilityPublic {
		h.push(ctx, GlobalTimeline, n.ID)
	}
}

// shouldFanoutToFollowers reports whether followers' home timelines should
// receive this note. specifiedノートは対象ユーザーにのみ届くため除外。
func shouldFanoutToFollowers(n *model.Note) bool {
	switch n.Visibility {
	case model.NoteVisibilityPublic, model.NoteVisibilityHome, model.NoteVisibilityFollowers:
		return true
	}
	return false
}

// fanoutToFollowers iterates the author's followers in pages and pushes the
// note id onto each follower's home timeline.
func (h *FanoutHook) fanoutToFollowers(ctx context.Context, authorID, noteID string) {
	const pageSize = 200
	offset := 0
	for {
		rows, err := h.followingRepo.ListFollowers(authorID, pageSize, offset)
		if err != nil {
			slog.Warn("fanoutToFollowers: list followers failed", "err", err, "author", authorID)
			return
		}
		if len(rows) == 0 {
			return
		}
		for _, f := range rows {
			h.push(ctx, HomeTimelineName(f.FollowerID), noteID)
		}
		if len(rows) < pageSize {
			return
		}
		offset += pageSize
	}
}

// push wraps Push with error logging.
func (h *FanoutHook) push(ctx context.Context, name Name, id string) {
	if err := h.fanout.Push(ctx, name, id, MaxTimelineLength); err != nil {
		slog.Warn("timeline push failed", "name", string(name), "id", id, "err", err)
	}
}
