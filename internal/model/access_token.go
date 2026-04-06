package model

import (
	"time"

	"github.com/lib/pq"
)

// AccessToken represents the `access_token` table.
type AccessToken struct {
	ID          string         `gorm:"column:id;type:varchar(32);primaryKey" json:"id"`
	LastUsedAt  *time.Time     `gorm:"column:lastUsedAt;type:timestamp with time zone" json:"lastUsedAt"`
	Token       string         `gorm:"column:token;type:varchar(128);not null" json:"token"`
	Session     *string        `gorm:"column:session;type:varchar(128)" json:"session"`
	Hash        string         `gorm:"column:hash;type:varchar(128);not null" json:"hash"`
	UserID      string         `gorm:"column:userId;type:varchar(32);not null" json:"userId"`
	AppID       *string        `gorm:"column:appId;type:varchar(32)" json:"appId"`
	Name        *string        `gorm:"column:name;type:varchar(128)" json:"name"`
	Description *string        `gorm:"column:description;type:varchar(512)" json:"description"`
	IconURL     *string        `gorm:"column:iconUrl;type:varchar(512)" json:"iconUrl"`
	Permission  pq.StringArray `gorm:"column:permission;type:varchar(64)[];default:'{}'" json:"permission"`
	Fetched     bool           `gorm:"column:fetched;default:false" json:"fetched"`

	// Relations
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (AccessToken) TableName() string { return "access_token" }
