package repository

import (
	"time"

	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// InstanceRepository provides data access for the `instance` table.
type InstanceRepository interface {
	Create(i *model.Instance) error
	FindByHost(host string) (*model.Instance, error)
	// FindManyByHosts returns instance rows matching any of the given hosts.
	// Used by entity packers to batch-resolve UserLite.Instance info on
	// timeline hot paths (#277). Empty input returns nil.
	FindManyByHosts(hosts []string) ([]*model.Instance, error)
	UpdateFields(host string, fields map[string]any) error
	IncrementCount(host, column string, delta int) error
	List(filter model.InstanceListFilter) ([]*model.Instance, error)
	// ListForRefresh returns instances whose metadata should be refreshed by
	// the periodic instance-refresh job (#393). Only live instances
	// (suspensionState = 'none' AND isNotResponding = false) whose
	// infoUpdatedAt is older than staleBefore (NULL counts as stale) are
	// returned. Ordered by infoUpdatedAt ASC NULLS FIRST so the oldest data
	// is refreshed first.
	ListForRefresh(staleBefore time.Time, limit int) ([]*model.Instance, error)
	// RecomputeFollowCounts refreshes followersCount / followingCount on
	// every instance row from the live `following` table. Currently nobody
	// keeps these counters in sync incrementally, so admin/overview's
	// federation pie chart gets all-zero slices. Running this on startup
	// gives the dashboard the right pie until incremental hooks land
	// (#421)。
	RecomputeFollowCounts() error
}

type instanceRepository struct {
	db *gorm.DB
}

// NewInstanceRepository creates a new InstanceRepository.
func NewInstanceRepository(db *gorm.DB) InstanceRepository {
	return &instanceRepository{db: db}
}

func (r *instanceRepository) Create(i *model.Instance) error {
	return r.db.Create(i).Error
}

func (r *instanceRepository) FindByHost(host string) (*model.Instance, error) {
	var inst model.Instance
	if err := r.db.Where("host = ?", host).First(&inst).Error; err != nil {
		return nil, err
	}
	return &inst, nil
}

func (r *instanceRepository) FindManyByHosts(hosts []string) ([]*model.Instance, error) {
	if len(hosts) == 0 {
		return nil, nil
	}
	var rows []*model.Instance
	if err := r.db.Where("host IN ?", hosts).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// UpdateFields applies a map of column → value updates to the instance row
// keyed by host. 集計列以外の任意フィールド更新に使う。
func (r *instanceRepository) UpdateFields(host string, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	return r.db.Model(&model.Instance{}).Where("host = ?", host).Updates(fields).Error
}

// IncrementCount adjusts a counter column on the instance row by delta.
// usersCount / notesCount / followingCount / followersCount などの集計列向け。
func (r *instanceRepository) IncrementCount(host, column string, delta int) error {
	return r.db.Model(&model.Instance{}).
		Where("host = ?", host).
		UpdateColumn(column, gorm.Expr("\""+column+"\" + ?", delta)).Error
}

// ListForRefresh implements the periodic metadata refresh query (#393).
func (r *instanceRepository) ListForRefresh(staleBefore time.Time, limit int) ([]*model.Instance, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	var rows []*model.Instance
	err := r.db.
		Where(`"suspensionState" = ?`, string(model.SuspensionStateNone)).
		Where(`"isNotResponding" = ?`, false).
		Where(`"infoUpdatedAt" IS NULL OR "infoUpdatedAt" < ?`, staleBefore).
		Order(`"infoUpdatedAt" ASC NULLS FIRST`).
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// List returns instances matching the filter, ordered by the requested sort.
func (r *instanceRepository) List(filter model.InstanceListFilter) ([]*model.Instance, error) {
	q := r.db.Model(&model.Instance{})
	if filter.Host != "" {
		q = q.Where("host ILIKE ?", "%"+filter.Host+"%")
	}
	if filter.Suspended != nil {
		if *filter.Suspended {
			q = q.Where("\"suspensionState\" <> ?", string(model.SuspensionStateNone))
		} else {
			q = q.Where("\"suspensionState\" = ?", string(model.SuspensionStateNone))
		}
	}
	if filter.NotResponding != nil {
		q = q.Where("\"isNotResponding\" = ?", *filter.NotResponding)
	}
	// federating / subscribing / publishingはhandler側のinstanceToMapと
	// 同じ式で判定する。false指定のときも反対条件でフィルタリングしないと、
	// レスポンス上のfederatingとfilterの意味論が食い違う (本家TSと同じ挙動)。
	if filter.Federating != nil {
		if *filter.Federating {
			// GORMはraw string中のOR条件を自動では括弧で囲まないため、
			// 他の.Where()とANDで連結したときに演算子優先順位で崩れる。
			// 同じ注意点はnote.go:407 / announcement.go:114と同様。
			q = q.Where("(\"followingCount\" > 0 OR \"followersCount\" > 0)")
		} else {
			q = q.Where("\"followingCount\" = 0 AND \"followersCount\" = 0")
		}
	}
	if filter.Subscribing != nil {
		if *filter.Subscribing {
			q = q.Where("\"followersCount\" > 0")
		} else {
			q = q.Where("\"followersCount\" = 0")
		}
	}
	if filter.Publishing != nil {
		if *filter.Publishing {
			q = q.Where("\"followingCount\" > 0")
		} else {
			q = q.Where("\"followingCount\" = 0")
		}
	}
	switch filter.SortBy {
	case "+host":
		q = q.Order("host ASC")
	case "-host":
		q = q.Order("host DESC")
	case "+notes":
		q = q.Order("\"notesCount\" ASC")
	case "-notes":
		q = q.Order("\"notesCount\" DESC")
	case "+users":
		q = q.Order("\"usersCount\" ASC")
	case "-users":
		q = q.Order("\"usersCount\" DESC")
	case "+following":
		q = q.Order("\"followingCount\" ASC")
	case "-following":
		q = q.Order("\"followingCount\" DESC")
	case "+followers":
		q = q.Order("\"followersCount\" ASC")
	case "-followers":
		q = q.Order("\"followersCount\" DESC")
	case "+firstRetrievedAt":
		q = q.Order("\"firstRetrievedAt\" ASC")
	default:
		// デフォルトは新しい順
		q = q.Order("\"firstRetrievedAt\" DESC")
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	q = q.Limit(limit).Offset(filter.Offset)
	var rows []*model.Instance
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// RecomputeFollowCounts recomputes the followersCount / followingCount
// columns of every instance row from the live `following` table. Used at
// startup to backfill stale zeros until incremental hooks land (#421)。
//
// `followersCount`: 当該リモートインスタンスの user を、ローカル user が
// 何人 follow しているか (= 我々から見た subscribe 元 instance 別)。
// `followingCount`: 当該リモートインスタンスの user が、ローカル user を
// 何人 follow しているか (= 我々を subscribe している側)。
//
// 命名は本家 Misskey と揃えてある。
//
// Reset → backfill の二段構え: 過去 follow が消えた host (= subquery に
// 出てこない) は subquery JOIN だと UPDATE 対象外になり、古い非ゼロ値が
// 永遠に残ってしまう (#421 Devin review)。先に全 instance を 0 にしてから
// 該当 host のみ再上書きすることで、follow を全部解除した instance も
// 確実に 0 へ戻す。
func (r *instanceRepository) RecomputeFollowCounts() error {
	if err := r.db.Exec(
		`UPDATE "instance" SET "followersCount" = 0, "followingCount" = 0`,
	).Error; err != nil {
		return err
	}
	// followersCount: COUNT of remote followees per host.
	const followers = `
UPDATE "instance" SET "followersCount" = c.cnt
FROM (
  SELECT u.host AS host, COUNT(*)::int AS cnt
  FROM "following" f
  JOIN "user" u ON f."followeeId" = u.id
  WHERE u.host IS NOT NULL
  GROUP BY u.host
) c
WHERE "instance".host = c.host`
	if err := r.db.Exec(followers).Error; err != nil {
		return err
	}
	// followingCount: COUNT of remote followers per host.
	const following = `
UPDATE "instance" SET "followingCount" = c.cnt
FROM (
  SELECT u.host AS host, COUNT(*)::int AS cnt
  FROM "following" f
  JOIN "user" u ON f."followerId" = u.id
  WHERE u.host IS NOT NULL
  GROUP BY u.host
) c
WHERE "instance".host = c.host`
	return r.db.Exec(following).Error
}
