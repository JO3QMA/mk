package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// RoleNotesQuery fetches notes by role via GORM.
type RoleNotesQuery struct {
	db *gorm.DB
}

// NewRoleNotesQuery creates a new RoleNotesQuery.
func NewRoleNotesQuery(db *gorm.DB) *RoleNotesQuery {
	return &RoleNotesQuery{db: db}
}

// ListByRole returns public notes from users assigned to the given role.
func (q *RoleNotesQuery) ListByRole(roleID string, limit int, sinceID, untilID string) ([]*model.Note, error) {
	query := q.db.Model(&model.Note{}).
		Joins(`JOIN "role_assignment" ON "role_assignment"."userId" = "note"."userId"`).
		Where(`"role_assignment"."roleId" = ?`, roleID).
		Where(`"note"."visibility" = 'public'`).
		Preload("User").
		Order(`"note"."id" DESC`).
		Limit(limit)

	if sinceID != "" {
		query = query.Where(`"note"."id" > ?`, sinceID)
	}
	if untilID != "" {
		query = query.Where(`"note"."id" < ?`, untilID)
	}

	var notes []*model.Note
	if err := query.Find(&notes).Error; err != nil {
		return nil, err
	}
	return notes, nil
}
