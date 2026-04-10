package model

import "time"

// Ad represents the `ad` table.
type Ad struct {
	ID        string    `gorm:"column:id;type:varchar(32);primaryKey" json:"id"`
	ExpiresAt time.Time `gorm:"column:expiresAt;type:timestamp with time zone;not null" json:"expiresAt"`
	StartsAt  time.Time `gorm:"column:startsAt;type:timestamp with time zone;not null;default:now()" json:"startsAt"`
	Place     string    `gorm:"column:place;type:varchar(32);not null" json:"place"`
	Priority  string    `gorm:"column:priority;type:varchar(32);not null;default:'middle'" json:"priority"`
	Ratio     int       `gorm:"column:ratio;type:integer;not null;default:1" json:"ratio"`
	URL       string    `gorm:"column:url;type:varchar(1024);not null" json:"url"`
	ImageURL  string    `gorm:"column:imageUrl;type:varchar(1024);not null" json:"imageUrl"`
	Memo      string    `gorm:"column:memo;type:varchar(8192);not null;default:''" json:"memo"`
	DayOfWeek int       `gorm:"column:dayOfWeek;type:integer;not null;default:0" json:"dayOfWeek"`
}

func (Ad) TableName() string { return "ad" }

// RegistrationTicket represents the `registration_ticket` table.
type RegistrationTicket struct {
	ID          string     `gorm:"column:id;type:varchar(32);primaryKey" json:"id"`
	Code        string     `gorm:"column:code;type:varchar(64);not null;uniqueIndex" json:"code"`
	ExpiresAt   *time.Time `gorm:"column:expiresAt;type:timestamp with time zone" json:"expiresAt"`
	CreatedByID *string    `gorm:"column:createdById;type:varchar(32)" json:"createdById"`
	UsedByID    *string    `gorm:"column:usedById;type:varchar(32)" json:"usedById"`
	UsedAt      *time.Time `gorm:"column:usedAt;type:timestamp with time zone" json:"usedAt"`
	PendingID   *string    `gorm:"column:pendingUserId;type:varchar(32)" json:"pendingUserId"`
}

func (RegistrationTicket) TableName() string { return "registration_ticket" }

// Relay represents the `relay` table.
type Relay struct {
	ID     string `gorm:"column:id;type:varchar(32);primaryKey" json:"id"`
	Inbox  string `gorm:"column:inbox;type:varchar(512);not null;uniqueIndex" json:"inbox"`
	Status string `gorm:"column:status;type:varchar(32);not null;default:'requesting'" json:"status"`
}

func (Relay) TableName() string { return "relay" }
