package model

import "time"

// PromoNote represents the `promo_note` table.
// admin が特定のノートをプロモーション (広告表示) として指定する。
type PromoNote struct {
	NoteID    string    `gorm:"column:noteId;type:varchar(32);primaryKey" json:"noteId"`
	ExpiresAt time.Time `gorm:"column:expiresAt;type:timestamp with time zone;not null" json:"expiresAt"`
	UserID    string    `gorm:"column:userId;type:varchar(32);not null" json:"userId"`
}

func (PromoNote) TableName() string { return "promo_note" }

// PromoRead represents the `promo_read` table.
// ユーザーが promo note を既読にしたことを記録する。
type PromoRead struct {
	ID     string `gorm:"column:id;type:varchar(32);primaryKey" json:"id"`
	UserID string `gorm:"column:userId;type:varchar(32);not null" json:"userId"`
	NoteID string `gorm:"column:noteId;type:varchar(32);not null" json:"noteId"`
}

func (PromoRead) TableName() string { return "promo_read" }
