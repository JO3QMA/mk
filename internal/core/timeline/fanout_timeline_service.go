// Package timeline provides Redis-backed timeline services.
//
// FanoutTimelineService maintains per-user/per-channel timeline lists in
// Redis. The model mirrors Misskey本家のFanoutTimelineService:
//   - 各タイムラインは "list:<name>" キーで管理されるRedisリスト
//   - LPUSHで新しいIDを先頭に追加
//   - LRANGEで取得し、since/untilでクライアント側フィルタ
package timeline

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shiroha-a/mk/internal/misc/id"
)

// Name represents a timeline key name.
// 例: "homeTimeline:USER_ID", "localTimeline", "userTimeline:USER_ID"
type Name string

// Well-known timeline name constructors. ローカル/グローバル/ホーム/ユーザーの
// 4種類はPhase 2 Step Eで実装。Misskey本家のホームタイムラインsuffixをそのまま流用。
const (
	LocalTimeline  Name = "localTimeline"
	GlobalTimeline Name = "globalTimeline"
	timelineKeyFmt      = "list:%s"
	defaultMaxLen       = 200
	// trimProbability defines how often we trim a timeline list to maxLen.
	// 全プッシュごとにLTRIMすると重いので、確率的に間引く。
	trimProbability = 0.1
	// recentInsertGracePeriod is how recent a note can be to skip the lindex check.
	recentInsertGracePeriod = 3 * time.Minute
)

// HomeTimelineName returns the home timeline list key for a user.
func HomeTimelineName(userID string) Name {
	return Name("homeTimeline:" + userID)
}

// UserTimelineName returns the per-user timeline list key.
func UserTimelineName(userID string) Name {
	return Name("userTimeline:" + userID)
}

// UserListTimelineName returns the user-list timeline list key.
func UserListTimelineName(listID string) Name {
	return Name("userListTimeline:" + listID)
}

// FanoutTimelineService manages Redis-backed timeline lists.
type FanoutTimelineService struct {
	client *redis.Client
	idGen  id.Generator
	// nowFn / randFn allow tests to inject deterministic values.
	nowFn  func() time.Time
	randFn func() float64
}

// NewFanoutTimelineService creates a new FanoutTimelineService bound to the
// timelines Redis database. idGen is used to extract a note's creation time
// from its ID for the late-insert ordering check.
func NewFanoutTimelineService(client *redis.Client, idGen id.Generator) *FanoutTimelineService {
	return &FanoutTimelineService{
		client: client,
		idGen:  idGen,
		nowFn:  time.Now,
		randFn: rand.Float64,
	}
}

// key returns the Redis key for a timeline name.
func (s *FanoutTimelineService) key(name Name) string {
	return fmt.Sprintf(timelineKeyFmt, name)
}

// Push appends id to the named timeline. maxLen caps the list size; values
// older than recentInsertGracePeriod fall back to a tail-comparison append.
//
// 戻り値はRedisエラー (nil if no-op or success)。
func (s *FanoutTimelineService) Push(ctx context.Context, name Name, noteID string, maxLen int) error {
	if maxLen <= 0 {
		maxLen = defaultMaxLen
	}
	noteTime, err := s.idGen.ParseTime(noteID)
	if err != nil {
		// 不正なIDはタイムラインに入れない (Misskey本家のidServiceでも例外を出す)
		return fmt.Errorf("invalid note id %q: %w", noteID, err)
	}

	key := s.key(name)
	// 直近 (3分以内) のノートはそのまま先頭に追加して、確率でtrim
	if s.nowFn().Sub(noteTime) <= recentInsertGracePeriod {
		if err := s.client.LPush(ctx, key, noteID).Err(); err != nil {
			return err
		}
		if s.randFn() < trimProbability {
			// LTrimの失敗はベストエフォートで握り潰す。
			// LPushは既に成功しており、サイズ制限はあくまで確率的なソフトリミット。
			_ = s.client.LTrim(ctx, key, 0, int64(maxLen-1)).Err()
		}
		return nil
	}

	// 古いノート: 既存末尾IDより新しい場合のみ追加
	tail, err := s.client.LIndex(ctx, key, -1).Result()
	if err == redis.Nil {
		// 空のタイムラインなら追加してOK
		return s.client.LPush(ctx, key, noteID).Err()
	}
	if err != nil {
		return err
	}
	tailTime, err := s.idGen.ParseTime(tail)
	if err != nil {
		// 末尾の値が壊れているなら諦めて追加する
		return s.client.LPush(ctx, key, noteID).Err()
	}
	if noteTime.After(tailTime) {
		return s.client.LPush(ctx, key, noteID).Err()
	}
	// より古いノートはスキップ
	return nil
}

// Get returns IDs from the timeline filtered by the given since/until window.
// 結果は新しいID順 (id降順)。limit<=0 のときは全件返す。
func (s *FanoutTimelineService) Get(ctx context.Context, name Name, untilID, sinceID string, limit int) ([]string, error) {
	ids, err := s.client.LRange(ctx, s.key(name), 0, -1).Result()
	if err != nil {
		return nil, err
	}
	return filterAndSort(ids, untilID, sinceID, limit), nil
}

// GetMulti retrieves IDs from multiple timelines in a single pipeline call.
// 戻り値の順序はnamesと同じ。
func (s *FanoutTimelineService) GetMulti(ctx context.Context, names []Name, untilID, sinceID string, limit int) ([][]string, error) {
	if len(names) == 0 {
		return nil, nil
	}
	pipe := s.client.Pipeline()
	cmds := make([]*redis.StringSliceCmd, 0, len(names))
	for _, n := range names {
		cmds = append(cmds, pipe.LRange(ctx, s.key(n), 0, -1))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, err
	}
	out := make([][]string, 0, len(cmds))
	for _, cmd := range cmds {
		// pipe.Exec が成功した時点で個々の cmd.Result() は基本的にエラーを返さない。
		// 万一エラーを返した場合は空スライスとして扱う。
		ids, _ := cmd.Result()
		out = append(out, filterAndSort(ids, untilID, sinceID, limit))
	}
	return out, nil
}

// Purge removes the named timeline list.
func (s *FanoutTimelineService) Purge(ctx context.Context, name Name) error {
	return s.client.Del(ctx, s.key(name)).Err()
}

// filterAndSort applies the since/until filter to a slice of IDs and returns
// them sorted in id-descending order, capped to limit if positive.
func filterAndSort(ids []string, untilID, sinceID string, limit int) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if untilID != "" && id >= untilID {
			continue
		}
		if sinceID != "" && id <= sinceID {
			continue
		}
		out = append(out, id)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(out)))
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
