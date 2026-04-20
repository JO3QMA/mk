package repository

import (
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
	// federating / subscribing / publishing は handler 側の instanceToMap と
	// 同じ式で判定する。false 指定のときも反対条件でフィルタリングしないと、
	// レスポンス上の federating と filter の意味論が食い違う (本家 TS と同じ挙動)。
	if filter.Federating != nil {
		if *filter.Federating {
			q = q.Where("\"followingCount\" > 0 OR \"followersCount\" > 0")
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
