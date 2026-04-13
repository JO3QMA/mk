package model

import "time"

// UsedUsername records usernames that have been used, preventing reuse.
type UsedUsername struct {
	Username  string    `gorm:"column:username;type:varchar(128);primaryKey"`
	CreatedAt time.Time `gorm:"column:createdAt;type:timestamp with time zone;not null;default:now()"`
}

func (UsedUsername) TableName() string { return "used_username" }
