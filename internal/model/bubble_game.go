package model

import (
	"time"

	"gorm.io/datatypes"
)

// BubbleGameRecord represents the `bubble_game_record` table.
type BubbleGameRecord struct {
	ID          string         `gorm:"column:id;type:varchar(32);primaryKey" json:"id"`
	UserID      string         `gorm:"column:userId;type:varchar(32);not null" json:"userId"`
	SeededAt    time.Time      `gorm:"column:seededAt;type:timestamp with time zone;not null" json:"seededAt"`
	Seed        string         `gorm:"column:seed;type:varchar(1024);not null" json:"seed"`
	GameVersion int            `gorm:"column:gameVersion;type:integer;not null" json:"gameVersion"`
	GameMode    string         `gorm:"column:gameMode;type:varchar(128);not null" json:"gameMode"`
	Score       int            `gorm:"column:score;type:integer;not null" json:"score"`
	Logs        datatypes.JSON `gorm:"column:logs;type:jsonb;default:'[]'" json:"logs"`
	IsVerified  bool           `gorm:"column:isVerified;default:false" json:"isVerified"`

	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (BubbleGameRecord) TableName() string { return "bubble_game_record" }
