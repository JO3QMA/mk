package model

import "time"

// Muting represents the `muting` table.
type Muting struct {
	ID        string     `gorm:"column:id;type:varchar(32);primaryKey" json:"id"`
	MuterID   string     `gorm:"column:muterId;type:varchar(32);not null" json:"muterId"`
	MuteeID   string     `gorm:"column:muteeId;type:varchar(32);not null" json:"muteeId"`
	ExpiresAt *time.Time `gorm:"column:expiresAt;type:timestamp with time zone" json:"expiresAt"`

	// Relations
	Muter *User `gorm:"foreignKey:MuterID" json:"muter,omitempty"`
	Mutee *User `gorm:"foreignKey:MuteeID" json:"mutee,omitempty"`
}

func (Muting) TableName() string { return "muting" }

// RenoteMuting represents the `renote_muting` table.
type RenoteMuting struct {
	ID      string `gorm:"column:id;type:varchar(32);primaryKey" json:"id"`
	MuterID string `gorm:"column:muterId;type:varchar(32);not null" json:"muterId"`
	MuteeID string `gorm:"column:muteeId;type:varchar(32);not null" json:"muteeId"`

	// Relations
	Muter *User `gorm:"foreignKey:MuterID" json:"muter,omitempty"`
	Mutee *User `gorm:"foreignKey:MuteeID" json:"mutee,omitempty"`
}

func (RenoteMuting) TableName() string { return "renote_muting" }
