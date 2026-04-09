package model

import (
	"time"

	"gorm.io/datatypes"
)

// RoleTarget defines how a role is assigned to users.
type RoleTarget string

const (
	RoleTargetManual      RoleTarget = "manual"
	RoleTargetConditional RoleTarget = "conditional"
)

// Role represents the `role` table.
type Role struct {
	ID              string         `gorm:"column:id;type:varchar(32);primaryKey" json:"id"`
	UpdatedAt       time.Time      `gorm:"column:updatedAt;not null" json:"updatedAt"`
	LastUsedAt      time.Time      `gorm:"column:lastUsedAt;not null" json:"lastUsedAt"`
	Name            string         `gorm:"column:name;type:varchar(256);not null" json:"name"`
	Description     string         `gorm:"column:description;type:varchar(1024);not null;default:''" json:"description"`
	Color           *string        `gorm:"column:color;type:varchar(256)" json:"color"`
	IconURL         *string        `gorm:"column:iconUrl;type:varchar(512)" json:"iconUrl"`
	Target          RoleTarget     `gorm:"column:target;type:role_target_enum;not null;default:'manual'" json:"target"`
	CondFormula     datatypes.JSON `gorm:"column:condFormula;type:jsonb;default:'{}'" json:"condFormula"`
	IsPublic        bool           `gorm:"column:isPublic;default:false" json:"isPublic"`
	AsBadge         bool           `gorm:"column:asBadge;default:false" json:"asBadge"`
	IsModerator     bool           `gorm:"column:isModerator;default:false" json:"isModerator"`
	IsAdministrator bool           `gorm:"column:isAdministrator;default:false" json:"isAdministrator"`
	IsExplorable    bool           `gorm:"column:isExplorable;default:false" json:"isExplorable"`
	DisplayOrder    int            `gorm:"column:displayOrder;default:0" json:"displayOrder"`
	Policies        datatypes.JSON `gorm:"column:policies;type:jsonb;default:'{}'" json:"policies"`

	PreserveAssignmentOnMoveAccount bool `gorm:"column:preserveAssignmentOnMoveAccount;default:false" json:"preserveAssignmentOnMoveAccount"`
	CanEditMembersByModerator       bool `gorm:"column:canEditMembersByModerator;default:false" json:"canEditMembersByModerator"`
}

func (Role) TableName() string { return "role" }

// RoleAssignment represents the `role_assignment` table.
type RoleAssignment struct {
	ID        string     `gorm:"column:id;type:varchar(32);primaryKey" json:"id"`
	UserID    string     `gorm:"column:userId;type:varchar(32);not null" json:"userId"`
	RoleID    string     `gorm:"column:roleId;type:varchar(32);not null" json:"roleId"`
	ExpiresAt *time.Time `gorm:"column:expiresAt" json:"expiresAt"`

	// Relations
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Role *Role `gorm:"foreignKey:RoleID" json:"role,omitempty"`
}

func (RoleAssignment) TableName() string { return "role_assignment" }
