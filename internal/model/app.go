package model

import (
	"time"

	"github.com/lib/pq"
)

// App represents the `app` table for MiAuth applications.
type App struct {
	ID          string         `gorm:"column:id;type:varchar(32);primaryKey" json:"id"`
	CreatedAt   time.Time      `gorm:"column:createdAt;type:timestamp with time zone;not null" json:"createdAt"`
	UserID      *string        `gorm:"column:userId;type:varchar(32)" json:"userId"`
	Secret      string         `gorm:"column:secret;type:varchar(64);not null" json:"secret"`
	Name        string         `gorm:"column:name;type:varchar(128);not null" json:"name"`
	Description string         `gorm:"column:description;type:varchar(512);not null" json:"description"`
	Permission  pq.StringArray `gorm:"column:permission;type:varchar(64)[];not null" json:"permission"`
	CallbackURL *string        `gorm:"column:callbackUrl;type:varchar(512)" json:"callbackUrl"`
}

func (App) TableName() string { return "app" }
