package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// AbuseReportRepository provides data access for the `abuse_user_report` table.
type AbuseReportRepository interface {
	Create(r *model.AbuseUserReport) error
	FindByID(id string) (*model.AbuseUserReport, error)
	List(resolved *bool, limit, offset int) ([]*model.AbuseUserReport, error)
	UpdateFields(id string, fields map[string]any) error
}

type abuseReportRepository struct {
	db *gorm.DB
}

func NewAbuseReportRepository(db *gorm.DB) AbuseReportRepository {
	return &abuseReportRepository{db: db}
}

func (r *abuseReportRepository) Create(report *model.AbuseUserReport) error {
	return r.db.Create(report).Error
}

func (r *abuseReportRepository) FindByID(id string) (*model.AbuseUserReport, error) {
	var report model.AbuseUserReport
	if err := r.db.Preload("TargetUser").Preload("Reporter").Preload("Assignee").
		Where("id = ?", id).First(&report).Error; err != nil {
		return nil, err
	}
	return &report, nil
}

func (r *abuseReportRepository) List(resolved *bool, limit, offset int) ([]*model.AbuseUserReport, error) {
	q := r.db.Preload("TargetUser").Preload("Reporter").Preload("Assignee").
		Order("id DESC")
	if resolved != nil {
		q = q.Where("resolved = ?", *resolved)
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	q = q.Limit(limit)
	if offset > 0 {
		q = q.Offset(offset)
	}
	var reports []*model.AbuseUserReport
	if err := q.Find(&reports).Error; err != nil {
		return nil, err
	}
	return reports, nil
}

func (r *abuseReportRepository) UpdateFields(id string, fields map[string]any) error {
	return r.db.Model(&model.AbuseUserReport{}).Where("id = ?", id).Updates(fields).Error
}

// ModerationLogRepository provides data access for the `moderation_log` table.
type ModerationLogRepository interface {
	Create(log *model.ModerationLog) error
	// CreateMany batch-inserts logs in one SQL statement (GORM slice-insert
	// uses a single multi-row INSERT). Used by Service.LogMany when bulk
	// admin operations want to record N entries without spawning N
	// goroutines + N round-trips (#671).
	CreateMany(logs []*model.ModerationLog) error
	List(limit, offset int) ([]*model.ModerationLog, error)
}

type moderationLogRepository struct {
	db *gorm.DB
}

func NewModerationLogRepository(db *gorm.DB) ModerationLogRepository {
	return &moderationLogRepository{db: db}
}

func (r *moderationLogRepository) Create(log *model.ModerationLog) error {
	return r.db.Create(log).Error
}

func (r *moderationLogRepository) CreateMany(logs []*model.ModerationLog) error {
	if len(logs) == 0 {
		return nil
	}
	return r.db.Create(&logs).Error
}

func (r *moderationLogRepository) List(limit, offset int) ([]*model.ModerationLog, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	q := r.db.Preload("User").Order("id DESC").Limit(limit)
	if offset > 0 {
		q = q.Offset(offset)
	}
	var logs []*model.ModerationLog
	if err := q.Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}
