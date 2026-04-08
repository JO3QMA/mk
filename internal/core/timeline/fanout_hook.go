package timeline

import (
	"context"
	"log/slog"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

// StreamingPublisher publishes a freshly created note to the WebSocket
// streaming pub/sub topics. パッケージ間の循環依存を避けるため interface で
// 受け取る (実装は internal/stream)。topic は Misskey 互換の論理名:
//   - "localTimeline"
//   - "globalTimeline"
//   - "homeTimeline:<userID>"
type StreamingPublisher interface {
	PublishNote(topic string, n *model.Note, author *model.User)
}

// FanoutHook implements note.TimelineFanoutHook by pushing newly-created notes
// onto the appropriate Redis timelines (home/local/global/user).
type FanoutHook struct {
	fanout        *FanoutTimelineService
	followingRepo repository.FollowingRepository
	publisher     StreamingPublisher
}

// NewFanoutHook constructs a FanoutHook.
func NewFanoutHook(fanout *FanoutTimelineService, followingRepo repository.FollowingRepository) *FanoutHook {
	return &FanoutHook{fanout: fanout, followingRepo: followingRepo}
}

// SetStreamingPublisher attaches a StreamingPublisher invoked alongside the
// Redis list fan-out so that WebSocket subscribers receive a push.
func (h *FanoutHook) SetStreamingPublisher(p StreamingPublisher) {
	h.publisher = p
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
	h.publishNote("homeTimeline:"+author.ID, n, author)
	if h.followingRepo != nil && shouldFanoutToFollowers(n) {
		h.fanoutToFollowers(ctx, author.ID, n.ID)
		h.fanoutStreamingToFollowers(author.ID, n, author)
	}

	// 3. ローカルタイムライン: ローカル投稿でvisibility=public/homeのみ
	if author.Host == nil && (n.Visibility == model.NoteVisibilityPublic || n.Visibility == model.NoteVisibilityHome) {
		h.push(ctx, LocalTimeline, n.ID)
		h.publishNote("localTimeline", n, author)
	}

	// 4. グローバルタイムライン: visibility=publicのみ
	if n.Visibility == model.NoteVisibilityPublic {
		h.push(ctx, GlobalTimeline, n.ID)
		h.publishNote("globalTimeline", n, author)
	}
}

// publishNote forwards the note to the StreamingPublisher when set. Best-
// effort wrapper used by OnNoteCreated and fanoutToFollowers.
func (h *FanoutHook) publishNote(topic string, n *model.Note, author *model.User) {
	if h.publisher == nil {
		return
	}
	h.publisher.PublishNote(topic, n, author)
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
// note id onto each follower's home timeline. WebSocket subscribers (if any)
// also receive the note via the streaming publisher.
func (h *FanoutHook) fanoutToFollowers(ctx context.Context, authorID, noteID string) {
	const pageSize = 200
	// note ID から model.Note を再取得しないため、publishNote には追加で
	// note と author を渡したい。が、現状 fanoutToFollowers は ID しか
	// 知らないので、呼び出し側 (OnNoteCreated) から model 参照を貰うのが
	// 自然。簡潔さのために、ここでは ID ベースのフォロワー集計だけ行い
	// streaming publish は OnNoteCreated 側でまとめて流す方針も取れる。
	// しかしフォロワー一覧の取得は重複させたくないので、publish も同じ
	// ループ内で実行する: 別 helper を作ってそちらに渡す。
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

// fanoutStreamingToFollowers publishes the note to the per-follower
// homeTimeline streaming topic. 別ループにすると DB を 2 回見る無駄があるが、
// publish は best-effort で fan-out が成功した後に走らせる方が安全 (publish
// 失敗が fan-out を巻き込まない)。フォロワー数が多い場合は将来 channel topic
// の broadcast を 1 つに統合する余地あり。
func (h *FanoutHook) fanoutStreamingToFollowers(authorID string, n *model.Note, author *model.User) {
	if h.publisher == nil || h.followingRepo == nil {
		return
	}
	const pageSize = 200
	offset := 0
	for {
		rows, err := h.followingRepo.ListFollowers(authorID, pageSize, offset)
		if err != nil {
			return
		}
		if len(rows) == 0 {
			return
		}
		for _, f := range rows {
			h.publisher.PublishNote("homeTimeline:"+f.FollowerID, n, author)
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
