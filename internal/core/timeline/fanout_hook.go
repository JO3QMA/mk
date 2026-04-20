package timeline

import (
	"context"
	"log/slog"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

// CacheLimits captures the four per-user timeline cache caps that
// `meta` exposes (Phase DB-compat issue #51 / parent #33). The values
// correspond directly to:
//
//   - LocalUserUserTimeline  → meta.perLocalUserUserTimelineCacheMax
//   - RemoteUserUserTimeline → meta.perRemoteUserUserTimelineCacheMax
//   - UserHomeTimeline       → meta.perUserHomeTimelineCacheMax
//   - UserListTimeline       → meta.perUserListTimelineCacheMax
//
// 0 や負の値はデフォルト値 (300/100/300/300) にフォールバックする。
type CacheLimits struct {
	LocalUserUserTimeline  int
	RemoteUserUserTimeline int
	UserHomeTimeline       int
	UserListTimeline       int
}

// MetaCacheLimitsProvider abstracts how FanoutHook reads the dynamic timeline
// cache caps from the meta table. 1 引数 1 戻り値の小さい interface にして
// テストで stub しやすくする。実装は通常 metaRepo を呼ぶだけ。
type MetaCacheLimitsProvider interface {
	CacheLimits() CacheLimits
}

// StreamingPublisher publishes a freshly created note to the WebSocket
// streaming pub/sub topics. パッケージ間の循環依存を避けるため interface で
// 受け取る (実装は internal/stream)。topic は Misskey 互換の論理名:
//   - "localTimeline"
//   - "globalTimeline"
//   - "homeTimeline:<userID>"
type StreamingPublisher interface {
	PublishNote(topic string, n *model.Note, author *model.User)
}

// UserListMemberLookup abstracts the query "which user lists contain this
// user?" for user list timeline fanout. 循環依存を避けるため interface で
// 受け取る (実装は repository.UserListRepository)。
type UserListMemberLookup interface {
	ListIDsByMember(userID string) ([]string, error)
}

// FanoutHook implements note.TimelineFanoutHook by pushing newly-created notes
// onto the appropriate Redis timelines (home/local/global/user).
type FanoutHook struct {
	fanout        *FanoutTimelineService
	followingRepo repository.FollowingRepository
	publisher     StreamingPublisher
	limits        MetaCacheLimitsProvider
	userListRepo  UserListMemberLookup
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

// SetCacheLimitsProvider attaches a MetaCacheLimitsProvider so that the
// per-timeline-kind cache caps come from the meta table at runtime. Without
// this setter the hook falls back to the package-level MaxTimelineLength
// constant for every timeline (legacy behaviour).
func (h *FanoutHook) SetCacheLimitsProvider(p MetaCacheLimitsProvider) {
	h.limits = p
}

// SetUserListRepo attaches a UserListMemberLookup so that note creation
// triggers push to userListTimeline:<listId> topics (#330).
func (h *FanoutHook) SetUserListRepo(r UserListMemberLookup) {
	h.userListRepo = r
}

// OnNoteCreated delivers the given note to user/home/local/global timelines.
// 配信失敗はログに記録するだけで上位に伝搬しない (ベストエフォート)。
func (h *FanoutHook) OnNoteCreated(n *model.Note, author *model.User) {
	if n == nil || author == nil {
		return
	}
	ctx := context.Background()

	// meta から動的な cache cap を 1 回だけ取得する。timeline 種別 4 つに
	// 渡すので per-push fetch ではなく per-create fetch にする (DB 呼び出し回数
	// を最小化)。limits が nil なら resolveCap は legacy デフォルト
	// (MaxTimelineLength) を返す。
	limits := h.fetchLimits()

	// 1. ユーザータイムライン (本人の投稿一覧)
	//    local user / remote user で別カラムを使う。
	userTimelineKind := UserTimelineKindLocal
	if author.Host != nil && *author.Host != "" {
		userTimelineKind = UserTimelineKindRemote
	}
	h.pushWithLimit(ctx, UserTimelineName(author.ID), n.ID, resolveCap(limits, userTimelineKind))

	// 2. ホームタイムライン: 投稿者本人 + フォロワー全員
	//    follower一覧の取得は1ページずつ繰り返し読みだす
	homeCap := resolveCap(limits, HomeTimelineKind)
	h.pushWithLimit(ctx, HomeTimelineName(author.ID), n.ID, homeCap)
	h.publishNote("homeTimeline:"+author.ID, n, author)
	if h.followingRepo != nil && shouldFanoutToFollowers(n) {
		h.fanoutToFollowers(ctx, author.ID, n.ID, homeCap)
		h.fanoutStreamingToFollowers(author.ID, n, author)
	}

	// 3. ローカルタイムライン: ローカル投稿でvisibility=publicのみ。
	// home visibilityはフォロワー向けなのでLTLには出さない (本家と同じ挙動)。
	if author.Host == nil && n.Visibility == model.NoteVisibilityPublic {
		h.pushWithLimit(ctx, LocalTimeline, n.ID, MaxTimelineLength)
		h.publishNote("localTimeline", n, author)
	}

	// 4. グローバルタイムライン: visibility=publicのみ
	if n.Visibility == model.NoteVisibilityPublic {
		h.pushWithLimit(ctx, GlobalTimeline, n.ID, MaxTimelineLength)
		h.publishNote("globalTimeline", n, author)
	}

	// 5. ユーザーリストタイムライン: 投稿者が属するリストへ配信
	if h.userListRepo != nil && shouldFanoutToFollowers(n) {
		listCap := resolveCap(limits, UserListTimelineKind)
		h.fanoutToUserLists(ctx, n, author, listCap)
	}
}

// fetchLimits returns the meta-derived cache caps, or zero values if no
// provider is wired (resolveCap then falls back to legacy defaults).
func (h *FanoutHook) fetchLimits() CacheLimits {
	if h.limits == nil {
		return CacheLimits{}
	}
	return h.limits.CacheLimits()
}

// TimelineKind enumerates the four per-user timeline categories that
// have a dedicated meta cache-cap column.
type TimelineKind int

// Sentinels for resolveCap. Local/global timelines do not have a dedicated
// meta column and continue to use the legacy MaxTimelineLength constant.
const (
	UserTimelineKindLocal TimelineKind = iota
	UserTimelineKindRemote
	HomeTimelineKind
	UserListTimelineKind
)

// resolveCap picks the right cap from limits and falls back to the per-kind
// legacy default when meta value is zero/negative. This keeps fresh installs
// (where the column hasn't been touched yet) on the documented Misskey
// defaults of 300/100/300/300.
func resolveCap(limits CacheLimits, kind TimelineKind) int {
	switch kind {
	case UserTimelineKindLocal:
		if limits.LocalUserUserTimeline > 0 {
			return limits.LocalUserUserTimeline
		}
		return 300
	case UserTimelineKindRemote:
		if limits.RemoteUserUserTimeline > 0 {
			return limits.RemoteUserUserTimeline
		}
		return 100
	case HomeTimelineKind:
		if limits.UserHomeTimeline > 0 {
			return limits.UserHomeTimeline
		}
		return 300
	case UserListTimelineKind:
		if limits.UserListTimeline > 0 {
			return limits.UserListTimeline
		}
		return 300
	}
	return MaxTimelineLength
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
// note id onto each follower's home timeline. homeCap は OnNoteCreated 側で
// meta から取得済みのものを再利用する (per-follower で fetch しない)。
func (h *FanoutHook) fanoutToFollowers(ctx context.Context, authorID, noteID string, homeCap int) {
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
			h.pushWithLimit(ctx, HomeTimelineName(f.FollowerID), noteID, homeCap)
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

// fanoutToUserLists pushes the note to all user lists that contain the author.
func (h *FanoutHook) fanoutToUserLists(ctx context.Context, n *model.Note, author *model.User, listCap int) {
	listIDs, err := h.userListRepo.ListIDsByMember(author.ID)
	if err != nil {
		slog.Warn("fanoutToUserLists: list lookup failed", "err", err, "author", author.ID)
		return
	}
	for _, listID := range listIDs {
		h.pushWithLimit(ctx, UserListTimelineName(listID), n.ID, listCap)
		h.publishNote("userListTimeline:"+listID, n, author)
	}
}

// pushWithLimit wraps Push with error logging and an explicit cap.
func (h *FanoutHook) pushWithLimit(ctx context.Context, name Name, id string, maxLen int) {
	if err := h.fanout.Push(ctx, name, id, maxLen); err != nil {
		slog.Warn("timeline push failed", "name", string(name), "id", id, "err", err)
	}
}
