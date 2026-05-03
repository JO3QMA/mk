package hashtags

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
	"github.com/shiroha-a/mk/internal/api/apierr"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// Handler handles hashtag-related API endpoints.
type Handler struct {
	db *gorm.DB
}

// NewHandler creates a new hashtags Handler.
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{db: db}
}

// List handles POST /api/hashtags/list.
func (h *Handler) List(c echo.Context) error {
	var req struct {
		Limit  int    `json:"limit"`
		Sort   string `json:"sort"`
		Offset int    `json:"offset"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "Invalid parameters.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Limit > 100 {
		req.Limit = 100
	}
	q := h.db.Model(&model.Hashtag{}).Order("\"mentionedUsersCount\" DESC").Limit(req.Limit)
	if req.Offset > 0 {
		q = q.Offset(req.Offset)
	}
	var tags []*model.Hashtag
	if err := q.Find(&tags).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	result := make([]map[string]any, 0, len(tags))
	for _, t := range tags {
		result = append(result, packTag(t))
	}
	return c.JSON(http.StatusOK, result)
}

// Search handles POST /api/hashtags/search.
func (h *Handler) Search(c echo.Context) error {
	var req struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := c.Bind(&req); err != nil || req.Query == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "query is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}
	var tags []*model.Hashtag
	if err := h.db.Where("name ILIKE ?", "%"+req.Query+"%").Limit(req.Limit).Find(&tags).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	names := make([]string, 0, len(tags))
	for _, t := range tags {
		names = append(names, t.Name)
	}
	return c.JSON(http.StatusOK, names)
}

// Show handles POST /api/hashtags/show.
func (h *Handler) Show(c echo.Context) error {
	var req struct {
		Tag string `json:"tag"`
	}
	if err := c.Bind(&req); err != nil || req.Tag == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "tag is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	var tag model.Hashtag
	if err := h.db.Where("name = ?", req.Tag).First(&tag).Error; err != nil {
		return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_HASHTAG", "No such hashtag.", "110ee688-193e-4a3a-9ecf-c167234e6f7d"))
	}
	return c.JSON(http.StatusOK, packTag(&tag))
}

// Trend handles POST /api/hashtags/trend.
//
// Misskey TS 互換: 直近 200 分 (10 分 × 20 bucket) の note 数を hashtag
// 別に集計し、frontend の MkMiniChart が描画できる時系列を返す。
//
// TS 上流 (HashtagService.getCharts) は Redis HyperLogLog で users 単位
// のユニーク数を count するが、mk-go では HLL を維持せず note テーブル
// から動的に集計する。
//
// mk-go の note テーブルは createdAt 列を持たず、aidx ID 先頭 8 文字に
// ms timestamp が埋め込まれている (`internal/misc/id/id.go` 参照)。
// 200 分前の cutoff prefix を生成して `id > cutoffID` で範囲フィルタを
// かける。tags 配列 (varchar(128)[]) には migration 000043 で GIN index
// を張ったので `<>` フィルタも index で効く。
//
// 取得した row は Go 側で aidx parse → bucket idx 計算 → tag/bucket 別
// の unique users を集計する。直近 200 分の対象 note 数は中規模インス
// タンスなら数千件オーダーで、Go 側の集計コストは無視できる。
//
// chart の bucket は新しいもの順 (index 0 = 直近 10 分、index 19 = 200 分前)。
// usersCount は TS と同じく `max(chart)` で算出する。
func (h *Handler) Trend(c echo.Context) error {
	const (
		bucketSeconds = 600 // 10 分
		bucketCount   = 20  // 過去 200 分
		topN          = 10
	)

	cutoff := time.Now().Add(-time.Duration(bucketSeconds*bucketCount) * time.Second)
	cutoffID := aidxCutoffPrefix(cutoff)

	type row struct {
		ID     string         `gorm:"column:id"`
		UserID string         `gorm:"column:userId"`
		Tags   pq.StringArray `gorm:"column:tags;type:varchar(128)[]"`
	}
	var rows []row
	if err := h.db.Raw(`
		SELECT "id", "userId", "tags"
		FROM "note"
		WHERE "id" > ?
		  AND "tags" <> ARRAY[]::varchar[]
	`, cutoffID).Scan(&rows).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}

	gen, err := id.NewGenerator("aidx")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	parser, ok := gen.(interface {
		ParseTime(string) (time.Time, error)
	})
	if !ok {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}

	// bucketUsers[tag][bucketIdx] = set of unique userId
	bucketUsers := make(map[string][bucketCount]map[string]struct{})
	totalUsers := make(map[string]map[string]struct{})

	now := time.Now()
	for _, r := range rows {
		ts, perr := parser.ParseTime(r.ID)
		if perr != nil {
			continue
		}
		secsAgo := now.Sub(ts).Seconds()
		if secsAgo < 0 || secsAgo >= float64(bucketSeconds*bucketCount) {
			continue
		}
		bucketIdx := int(secsAgo) / bucketSeconds
		for _, tag := range r.Tags {
			if tag == "" {
				continue
			}
			buckets, exists := bucketUsers[tag]
			if !exists {
				for i := range buckets {
					buckets[i] = map[string]struct{}{}
				}
				bucketUsers[tag] = buckets
			}
			bucketUsers[tag][bucketIdx][r.UserID] = struct{}{}

			if _, exists := totalUsers[tag]; !exists {
				totalUsers[tag] = map[string]struct{}{}
			}
			totalUsers[tag][r.UserID] = struct{}{}
		}
	}

	// ranking: total users 降順、同点は tag name 昇順 (deterministic)
	type ranked struct {
		tag   string
		total int
	}
	rankings := make([]ranked, 0, len(totalUsers))
	for tag, users := range totalUsers {
		rankings = append(rankings, ranked{tag: tag, total: len(users)})
	}
	sort.Slice(rankings, func(i, j int) bool {
		if rankings[i].total != rankings[j].total {
			return rankings[i].total > rankings[j].total
		}
		return rankings[i].tag < rankings[j].tag
	})
	if len(rankings) > topN {
		rankings = rankings[:topN]
	}

	result := make([]map[string]any, 0, len(rankings))
	for _, rk := range rankings {
		chart := make([]int, bucketCount)
		buckets := bucketUsers[rk.tag]
		maxUsers := 0
		for i := 0; i < bucketCount; i++ {
			n := len(buckets[i])
			chart[i] = n
			if n > maxUsers {
				maxUsers = n
			}
		}
		result = append(result, map[string]any{
			"tag":        rk.tag,
			"chart":      chart,
			"usersCount": maxUsers,
		})
	}
	return c.JSON(http.StatusOK, result)
}

// aidxCutoffPrefix builds the smallest aidx-style ID at the given time
// for use as a `WHERE id > ?` cutoff in note range scans. mk-go の note
// は created_at 列を持たず aidx ID 先頭 8 文字に ms timestamp を埋め込
// んでいるので、prefix だけ生成すれば lexicographic に正しく比較できる。
//
// time2000 と base36 padding の生成は internal/misc/id/id.go の aidxGen
// と同じ規約。Generate() を再利用するとカウンタが 1 進んでしまうため
// ここでは独自に prefix のみ作る。
func aidxCutoffPrefix(t time.Time) string {
	const time2000 int64 = 946684800000
	ms := t.UnixMilli() - time2000
	if ms < 0 {
		ms = 0
	}
	timePart := fmt.Sprintf("%08s", strconv.FormatInt(ms, 36))
	if len(timePart) > 8 {
		timePart = timePart[len(timePart)-8:]
	}
	// 後続 8 文字 (nodeID + counter) は最小値で揃え、当該時刻の最小 ID を作る
	return timePart + "00000000"
}

// Users handles POST /api/hashtags/users.
func (h *Handler) Users(c echo.Context) error {
	var req struct {
		Tag   string `json:"tag"`
		Limit int    `json:"limit"`
	}
	if err := c.Bind(&req); err != nil || req.Tag == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "tag is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	// ハッシュタグを使ったユーザー一覧 (簡易版: 空配列)
	return c.JSON(http.StatusOK, []any{})
}

func packTag(t *model.Hashtag) map[string]any {
	return map[string]any{
		"tag":                       t.Name,
		"mentionedUsersCount":       t.MentionedUsersCount,
		"mentionedLocalUsersCount":  t.MentionedLocalUsersCount,
		"mentionedRemoteUsersCount": t.MentionedRemoteUsersCount,
		"attachedUsersCount":        t.AttachedUsersCount,
		"attachedLocalUsersCount":   t.AttachedLocalUsersCount,
		"attachedRemoteUsersCount":  t.AttachedRemoteUsersCount,
	}
}
