package model

import "gorm.io/datatypes"

// Signin represents the `signin` table for login history.
type Signin struct {
	ID      string         `gorm:"column:id;type:varchar(32);primaryKey" json:"id"`
	UserID  string         `gorm:"column:userId;type:varchar(32);not null" json:"userId"`
	IP      string         `gorm:"column:ip;type:varchar(128);not null" json:"ip"`
	Headers datatypes.JSON `gorm:"column:headers;type:jsonb;not null" json:"headers"`
	Success bool           `gorm:"column:success;not null;default:true" json:"success"`
}

func (Signin) TableName() string { return "signin" }
