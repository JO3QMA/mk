package model

import (
	"time"

	"github.com/lib/pq"
	"gorm.io/datatypes"
)

// RegistryItem represents the `registry_item` table.
// フロントエンドのユーザー設定 (テーマ、UI設定等) を永続化する。
type RegistryItem struct {
	ID        string         `gorm:"column:id;type:varchar(32);primaryKey" json:"id"`
	UpdatedAt time.Time      `gorm:"column:updatedAt;not null" json:"updatedAt"`
	UserID    string         `gorm:"column:userId;type:varchar(32);not null" json:"userId"`
	Key       string         `gorm:"column:key;type:varchar(1024);not null" json:"key"`
	Value     datatypes.JSON `gorm:"column:value;type:jsonb;default:'{}'" json:"value"`
	Scope     pq.StringArray `gorm:"column:scope;type:varchar(1024)[];default:'{}'" json:"scope"`
	Domain    *string        `gorm:"column:domain;type:varchar(512)" json:"domain"`
}

func (RegistryItem) TableName() string { return "registry_item" }
